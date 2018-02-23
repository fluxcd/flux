package git

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"sync"
	"time"

	"github.com/go-kit/kit/log"
	"golang.org/x/crypto/ssh"
	gogit "gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	gitssh "gopkg.in/src-d/go-git.v4/plumbing/transport/ssh"
)

const (
	DefaultCloneTimeout = 2 * time.Minute
	privateKeyFileMode  = os.FileMode(0400)
	ChartsChangesClone  = "charts_sync_gitclone"
)

var (
	ErrNoChanges    = errors.New("no changes made in repo")
	ErrNoChartsDir  = errors.New("no Charts dir provided")
	ErrNoRepo       = errors.New("no repo provided")
	ErrNoRepoCloned = errors.New("no repo cloned")
)

type GitRemoteConfig struct {
	URL    string `json:"url"`
	Branch string `json:"branch"`
	Path   string `json:"path"`
}

// Checkout is a local clone of the remote repo.
type Checkout struct {
	Logger log.Logger
	Config GitRemoteConfig // remote repo info provided by the user
	//	privateKeyPath string
	auth     *gitssh.PublicKeys
	Dir      string            // directory where the repo was cloned
	repo     *gogit.Repository // cloned repo info
	worktree *gogit.Worktree
	sync.RWMutex
}

// NewGitRemoteConfig ... sets up git repo configuration.
func NewGitRemoteConfig(url, branch, path string) (GitRemoteConfig, error) {
	if len(url) == 0 {
		return GitRemoteConfig{}, errors.New("git repo URL must be provided")
	}
	if len(branch) == 0 {
		branch = "master"
	}
	if len(path) == 0 || (len(path) != 0 && path[0] == '/') {
		return GitRemoteConfig{}, errors.New("git subdirectory (--git-charts-path) must be probided and cannot have leading forward slash")
	}

	return GitRemoteConfig{
		URL:    url,
		Branch: branch,
		Path:   path,
	}, nil
}

// NewCheckout ... creates a Checkout instance
//		populates also private fields relating to ssh authentication
func NewCheckout(logger log.Logger, config GitRemoteConfig, auth *gitssh.PublicKeys) *Checkout {
	return &Checkout{
		Logger: logger,
		Config: config,
		//	privateKeyPath: privateKeyPath,
		auth: auth,
	}
}

// Clone creates a local clone of a remote repo and
// checks out the relevant branch
//		subdir reflects the purpose of the clone:
//																		* acting on Charts changes (syncing the cluster when there were only commits
//																		  in the Charts parts of the repo which did not trigger Custom Resource changes)
func (ch *Checkout) Clone(ctx context.Context, cloneSubdir string) error {
	ch.Lock()
	defer ch.Unlock()

	if ch.Config.URL == "" {
		return ErrNoRepo
	}

	fmt.Println("\t\tstage 1")

	repoDir, err := ioutil.TempDir(os.TempDir(), cloneSubdir)
	if err != nil {
		return err
	}
	ch.Dir = repoDir

	fmt.Println("\t\tstage 2")

	repo, err := gogit.PlainClone(repoDir, false, &gogit.CloneOptions{
		URL:  ch.Config.URL,
		Auth: ch.auth,
	})
	//if err != nil && err != gogit.ErrRepositoryAlreadyExists {
	if err != nil {
		return err
	}

	fmt.Println("\t\tstage 3")

	wt, err := repo.Worktree()
	if err != nil {
		return err
	}

	fmt.Println("\t\tstage 4")

	br := ch.Config.Branch
	err = wt.Checkout(&gogit.CheckoutOptions{
		Branch: plumbing.ReferenceName(fmt.Sprintf("refs/heads/%s", br)),
	})
	if err != nil {
		return err
	}

	fmt.Println("\t\tstage 5")

	ch.repo = repo
	ch.worktree = wt

	return nil
}

// Cleanup ... removes the temp repo directory
func (ch *Checkout) Cleanup() {
	ch.Lock()
	defer ch.Unlock()

	if ch.Dir != "" {
		err := os.RemoveAll(ch.Dir)
		if err != nil {
			ch.Logger.Log("error", err.Error())
		}
	}
	ch.Dir = ""
	ch.repo = nil
	ch.worktree = nil
}

/*
// ChangedCharts makes a new git pull and determines which charts changed
// ChangedCharts method does a git pull and finds the latest revisison
// Among suppplied custom resources it finds the ones whose revision is different
// Charts related to these custom resources need to be released
func (ch *Checkout) ChangedCharts(crs []ifv1.FluxHelmResource) ([]ifv1.FluxHelmResource, error) {
	ch.Lock()
	defer ch.Unlock()

	rev, err := ch.getRevision()
	if err != nil {
		return nil, err
	}

	if err := ch.pull(); err != nil {
		return nil, err
	}

		rev, err := ch.getRevision()
		if err != nil {
			return nil, err
		}

		var crForUpdate []ifv1.FluxHelmResource
		for _, cr := range crs {
				crstatus := cr.Status.Revision
				// When a new Custom Resource is created, Status.Revision
				if crstatus != "" && crstatus != rev {
					crForUpdate = append(crForUpdate, cr)
				}
		}

	return []ifv1.FluxHelmResource{}, nil
}
*/

// CloneLoop ... tries until the repo is cloned
func (ch *Checkout) CloneLoop(cloneSubdir string, chanDone chan struct{}) {
	return
}

// GetRepoAuth ... provides git repo authentication based on private ssh key
func GetRepoAuth(k8sSecretVolumeMountPath, k8sSecretDataKey string) (*gitssh.PublicKeys, error) {
	privateKeyPath := path.Join(k8sSecretVolumeMountPath, k8sSecretDataKey)
	fileInfo, err := os.Stat(privateKeyPath)
	switch {
	case os.IsNotExist(err):
		return &gitssh.PublicKeys{}, err
	case err != nil:
		return &gitssh.PublicKeys{}, err
	case fileInfo.Mode() != privateKeyFileMode:
		if err := os.Chmod(privateKeyPath, privateKeyFileMode); err != nil {
			return &gitssh.PublicKeys{}, err
		}
	default:
	}

	sshKey, err := ioutil.ReadFile(privateKeyPath)
	if err != nil {
		return nil, err
	}
	signer, err := ssh.ParsePrivateKey([]byte(sshKey))
	if err != nil {
		return nil, err
	}
	auth := &gitssh.PublicKeys{User: "git", Signer: signer}

	return auth, nil
}

func (ch *Checkout) Pull() error {
	w := ch.worktree
	if w == nil {
		return ErrNoRepoCloned
	}
	err := w.Pull(&gogit.PullOptions{
		RemoteName: "origin",
		Auth:       ch.auth,
	})
	if err != nil && err != gogit.NoErrAlreadyUpToDate {
		return err
	}
	return nil
}

/*
// getRevision returns string representation of the revision hash
func (ch *Checkout) getRevision() (string, error) {
	if ch.repo == nil {
		return "", ErrNoRepoCloned
	}
	ref, err := ch.repo.Head()
	if err != nil {
		return "", err
	}
	rev := ref.Hash().String()
	return rev, nil
}
*/
