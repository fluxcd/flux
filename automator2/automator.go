package automator

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/weaveworks/fluxy/platform"
	"github.com/weaveworks/fluxy/platform/kubernetes"
	"github.com/weaveworks/fluxy/registry"
)

// Automator implements continuous deployment.
type Automator struct {
	cfg    Config
	mtx    sync.RWMutex
	active map[namespaceService]bool
}

type namespaceService struct{ namespace, serviceName string }

// Automate enables automation for the identified service.
func (a *Automator) Automate(namespace, serviceName string) {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	a.active[namespaceService{namespace, serviceName}] = true
}

// Deautomate disables automation for the identified service.
func (a *Automator) Deautomate(namespace, serviceName string) {
	a.mtx.Lock()
	defer a.mtx.Unlock()
	delete(a.active, namespaceService{namespace, serviceName})
}

// Loop invokes Poll every time the passed tick chan fires.
// Create a time.NewTicker and pass its C chan to this function.
// This method blocks until the tick chan is closed.
// You probably want to run it in its own goroutine.
func (a *Automator) Loop(tick <-chan time.Time) {
	for range tick {
		a.Poll()
	}
}

// Poll performs the following steps.
//
//    1. Poll the platform for all automation-enabled services,
//    2. Collect all the container images currently running,
//    3. Poll the relevant image repos and get the current images.
//    4. Compare images from steps 2 and 3; any mismatch requires a release.
//    5. The release process is one shot for all services:
//       a. Clone the config repo,
//       b. Identify all files containing any of the mismatched images,
//       c. Config update all, mutating them locally,
//       d. Commit and push all, and finally
//       e. Perform a release for each in a blocking fashion.
//
func (a *Automator) Poll() {
	a.cfg.Logger.Log("automator", "Poll start")
	defer a.cfg.Logger.Log("automator", "Poll complete")

	// Poll the platform for all automation-enabled services.
	active := func() (res []namespaceService) {
		a.mtx.RLock()
		defer a.mtx.RUnlock()
		for ns := range a.active {
			res = append(res, ns)
		}
		return res
	}()

	var services []platform.Service
	for _, ns := range active {
		service, err := a.cfg.Platform.Service(ns.namespace, ns.serviceName)
		if err != nil {
			a.cfg.Logger.Log("namespace", ns.namespace, "service", ns.serviceName, "err", err)
			continue
		}
		services = append(services, service)
	}
	if len(services) <= 0 {
		a.cfg.Logger.Log("notice", "no automated services detected on platform")
		return
	}

	// Collect all the container images currently running.
	var images []registry.Image
	for _, service := range services {
		// TODO(pb): If the service is running multiple images, we actually need
		// to know them. So, we need to amend platform.Service to track that
		// correctly. platform.Service.Image needs to become Images, with a
		// String method that produces human-readable output for fluxctl.
		//
		// Until then, this is a little hack to detect the presence of multiple
		// images without doing string comparisons.
		image := registry.ParseImage(service.Image)
		if image.Tag == "" {
			a.cfg.Logger.Log("service", service.Name, "image", service.Image, "err", "unparseable image; skipping")
			continue
		}
		images = append(images, image)
	}
	if len(images) <= 0 {
		a.cfg.Logger.Log("warning", "no images found from platform services")
		return
	}

	// Poll the relevant image repos and get the current images.
	// Compare images; any mismatch requires a release.
	needRelease := map[registry.Image]registry.Image{} // map of current to latest
	for _, image := range images {
		repository, err := a.cfg.Registry.GetRepository(image.Repository())
		if err != nil {
			a.cfg.Logger.Log("image", image.String(), "err", err)
			continue
		}
		if len(repository.Images) <= 0 {
			a.cfg.Logger.Log("repository", repository.Name, "err", "no images found")
			continue
		}
		if image.String() == repository.Images[0].String() {
			continue // already have latest
		}
		needRelease[image] = repository.Images[0]
	}
	if len(needRelease) <= 0 {
		a.cfg.Logger.Log("notice", "no images are out-of-date")
		return
	}

	// Now we begin the release process, which is one-shot for all services.
	// Clone the config repo.
	tmpdir, err := a.cfg.Repo.Clone()
	if err != nil {
		a.cfg.Logger.Log("repo", a.cfg.Repo.Path, "err", err)
		return
	}
	defer os.RemoveAll(tmpdir)

	// Identify all files containing any of the images that need a release.
	// Config update all, mutating them locally.
	updatedFiles := map[string]struct{}{} // a set, because we may have dupes
	for currentImage, latestImage := range needRelease {
		// Find any file referencing this current image.
		basepath := filepath.Join(tmpdir, a.cfg.Repo.Path)
		needle := currentImage.String()
		files := findFilesFor(basepath, needle)
		if len(files) <= 0 {
			a.cfg.Logger.Log("image", currentImage.String(), "err", "no files found referencing this image; strange!")
			continue
		}

		// Update all of them to point to the corresponding latest image.
		for _, file := range files {
			if err := updatePodController(file, latestImage.String()); err != nil {
				a.cfg.Logger.Log("file", file, "from", currentImage.String(), "to", latestImage.String(), "err", err)
				continue
			}
			updatedFiles[file] = struct{}{}
		}
	}
	if len(updatedFiles) <= 0 {
		a.cfg.Logger.Log("warning", "no files were updated; strange!")
		return
	}

	// Commit and push all those files.
	// (Treating all errors as fatal: strictly necessary?)
	for file := range updatedFiles {
		if err := gitCommit(file); err != nil {
			a.cfg.Logger.Log("commit", file, "err", err)
			return
		}
	}
	if err := gitPush(tmpdir); err != nil {
		a.cfg.Logger.Log("err", err)
		return
	}

	// Each file maps to a service.
	// Perform a release for each, in series.
	for file := range updatedFiles {
		var (
			namespace   = "???" // TODO(pb)
			serviceName = "???" // TODO(pb)
		)
		logger := log.NewContext(a.cfg.Logger).With("namespace", namespace, "service", serviceName)
		newDef, err := ioutil.ReadFile(file)
		if err != nil {
			logger.Log("release", "FAILED", "err", err)
			continue
		}
		if err := a.cfg.Platform.Release(namespace, serviceName, newDef, a.cfg.UpdatePeriod); err != nil {
			logger.Log("release", "FAILED", "err", err)
			continue
		}
		logger.Log("release", "succeeded")
	}
}

func findFilesFor(path string, imageStr string) []string {
	var res []string
	filepath.Walk(path, func(tgt string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil // recurse into dirs
		}
		if ext := filepath.Ext(tgt); ext != ".yaml" && ext != ".yml" {
			return nil // only target YAML files
		}
		if !fileContains(tgt, imageStr) {
			return nil
		}
		res = append(res, tgt)
		return nil
	})
	return res
}

func fileContains(filename string, s string) bool {
	buf, err := ioutil.ReadFile(filename)
	if err != nil {
		return false
	}
	if strings.Contains(string(buf), s) {
		return true
	}
	return false
}

func updatePodController(filename string, newImageName string) error {
	fi, err := os.Stat(filename)
	if err != nil {
		return err
	}
	in, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	out, err := kubernetes.UpdatePodController(in, newImageName, ioutil.Discard)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filename, out, fi.Mode())
}

func gitCommit(filename string) error {
	return nil // TODO(pb)
}

func gitPush(fromDir string) error {
	return nil // TODO(pb)
}
