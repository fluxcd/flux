package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"golang.org/x/crypto/ssh"

	gitssh "gopkg.in/src-d/go-git.v4/plumbing/transport/ssh"

	"github.com/go-kit/kit/log"
	"github.com/weaveworks/flux/integrations/helm/git"
)

var (
	url            string
	branch         string
	chartsPath     string
	privateKeyPath string

	logger log.Logger
)

func init() {
	flag.StringVar(&url, "git-url", "git@github.com:sambooo/flux-example.git", "git repo url")
	flag.StringVar(&branch, "git-branch", "master", "git branch")
	flag.StringVar(&chartsPath, "git-charts-path", "charts", "git charts path")
	flag.StringVar(&privateKeyPath, "private-key-path", "~/.ssh/id_rsa", "ssh private key path")

	logger = log.With(
		log.NewLogfmtLogger(os.Stderr),
		"ts", log.DefaultTimestampUTC,
		"caller", log.DefaultCaller,
	)
}

func main() {
	flag.Parse()
	keyBytes, err := ioutil.ReadFile(privateKeyPath)
	if err != nil {
		panic(err)
	}
	key, err := ssh.ParsePrivateKey(keyBytes)
	if err != nil {
		panic(err)
	}

	gitRemoteConfig, err := git.NewGitRemoteConfig(url, branch, chartsPath)
	if err != nil {
		panic(err)
	}
	logger.Log("gitRemoteConfig", fmt.Sprintf("%#v", gitRemoteConfig))

	checkout := git.NewCheckout(logger, gitRemoteConfig, &gitssh.PublicKeys{
		User: "git", Signer: key,
	})
	defer checkout.Cleanup()

	for {
		logger.Log("msg", "attempting to clone repo")
		ctx, cancel := context.WithTimeout(context.Background(), git.DefaultCloneTimeout)
		err := checkout.Clone(ctx, "git-test")
		cancel()
		if err == nil {
			logger.Log("msg", "git clone succeeded")
			break
		}
		logger.Log("err", err)
		time.Sleep(1 * time.Second)
	}
}
