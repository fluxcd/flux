// +build integration_test

package test

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"testing"
)

const (
	fluxImage         = "quay.io/weaveworks/flux:latest"
	fluxOperatorImage = "quay.io/weaveworks/helm-operator:latest"
	fluxNamespace     = "flux"
	helmFluxRelease   = "cd"
)

type (
	// setup is generally concerned with things that apply globally, and don't
	// depend on harness state.
	workdir struct {
		root          string
		sshKnownHosts string
	}

	setup struct {
		workdir
		profile   string
		clusterIP string
		clusterAPI
		kubectlAPI
		helmAPI
	}
)

var (
	global setup
)

func newsetup(profile string) *setup {
	dir, err := ioutil.TempDir("", "fluxtest")
	if err != nil {
		log.Fatalf("Error creating tempdir: %v", err)
	}

	return &setup{
		workdir: workdir{root: dir},
		profile: profile,
	}
}

func (s *setup) clean() error {
	return os.RemoveAll(s.workdir.root)
}

func (w workdir) knownHostsPath() string {
	return filepath.Join(w.root, "ssh-known-hosts")
}

func (s *setup) must(err error) {
	if err != nil {
		log.Fatalf("%s", err)
	}
}

func (s *setup) ssh(namespace string) {
	knownHostsContent := execNoErr(context.TODO(), nil, "ssh-keyscan", s.clusterIP)
	ioutil.WriteFile(s.knownHostsPath(), []byte(knownHostsContent), 0600)

	s.kubectlAPI.create("", "namespace", namespace)

	secretName := "flux-git-deploy"
	s.kubectlAPI.delete(namespace, "secret", secretName)
	s.must(s.kubectlAPI.create(namespace, "secret", "generic", secretName, "--from-file",
		fmt.Sprintf("identity=%s", s.sshKeyPath())))

	configMapName := "ssh-known-hosts"
	s.kubectlAPI.delete(namespace, "configmap", configMapName)
	s.must(s.kubectlAPI.create(namespace, "configmap", configMapName, "--from-file",
		fmt.Sprintf("known_hosts=%s", s.knownHostsPath())))
}

func setupPath() {
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatalf("cannot get working directory: %v", err)
	}
	envpath := os.Getenv("PATH")
	if envpath == "" {
		envpath = filepath.Join(cwd, "bin")
	} else {
		envpath = filepath.Join(cwd, "bin") + ":" + envpath
	}
	os.Setenv("PATH", envpath)
}

func TestMain(m *testing.M) {
	var (
		flagKeepWorkdir = flag.Bool("keep-workdir", false,
			"don't delete workdir on exit")
		flagStartMinikube = flag.Bool("start-minikube", false,
			"start minikube (or delete and start if it already exists)")
		flagMinikubeProfile = flag.String("minikube-profile", "minikube",
			"minikube profile to use, don't change until we have a fix for https://github.com/kubernetes/minikube/issues/2717")
	)
	flag.Parse()
	log.Printf("Testing with keep-workdir=%v, start-minikube=%v, minikube-profile=%v",
		*flagKeepWorkdir, *flagStartMinikube, *flagMinikubeProfile)

	setupPath()

	global = *newsetup(*flagMinikubeProfile)
	if !*flagKeepWorkdir {
		defer global.clean()
	}

	minikube := mustNewMinikube(stdLogger{}, *flagMinikubeProfile)
	if *flagStartMinikube {
		minikube.delete()
		minikube.start()
	}

	global.clusterAPI = minikube
	global.clusterIP = minikube.nodeIP()
	global.kubectlAPI = mustNewKubectl(stdLogger{}, *flagMinikubeProfile)
	global.helmAPI = mustNewHelm(stdLogger{}, *flagMinikubeProfile,
		global.workdir.root, global.kubectlAPI)

	global.loadDockerImage(fluxImage)
	global.loadDockerImage(fluxOperatorImage)

	global.ssh(fluxNamespace)

	// Make sure that if helm flux is sitting around due to a previous failed
	// test, it won't interfere with upcoming tests.
	global.helmAPI.delete(helmFluxRelease, true)

	os.Exit(m.Run())
}
