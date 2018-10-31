package test

import (
	"context"
	"regexp"
	"time"

	"github.com/Masterminds/semver"
)

const (
	k8sPreferredVersion = "v1.10.5"
	k8sVersionRange     = ">1.9.4" // need post-1.9.4 due to https://github.com/kubernetes/kubernetes/issues/61076
)

type (
	kubectlTool struct {
		profile string
	}

	kubectlAPI interface {
		kubeVersion() string
		create(namespace string, args ...string) error
		delete(namespace string, args ...string) error
		cli() clicmd
	}

	kubectl struct {
		kt kubectlTool
		lg logger
	}
)

func (kt kubectlTool) common() []string {
	return []string{"kubectl", "--context", kt.profile}
}

func (kt kubectlTool) versionCmd() []string {
	return append(kt.common(), []string{"version"}...)
}

func (kt kubectlTool) createCmd(namespace string) []string {
	return append(kt.common(), []string{"--namespace", namespace, "create"}...)
}

func (kt kubectlTool) deleteCmd(namespace string) []string {
	return append(kt.common(), []string{"--namespace", namespace, "delete"}...)
}

func newKubectlTool(profile string) (*kubectlTool, error) {
	return &kubectlTool{profile: profile}, nil
}

func mustNewKubectl(lg logger, profile string) kubectl {
	kt, err := newKubectlTool(profile)
	if err != nil {
		lg.Fatalf("%v", err)
	}

	k := kubectl{kt: *kt, lg: lg}
	runningVersion := k.kubeVersion()
	versionCheck, _ := semver.NewConstraint(k8sVersionRange)
	v := semver.MustParse(runningVersion)
	if !versionCheck.Check(v) {
		lg.Fatalf("running kubernetes version is %s but only %s is supported"+
			", see https://github.com/kubernetes/kubernetes/issues/61076",
			runningVersion, k8sVersionRange)
	}
	return k
}

func (k kubectl) cli() clicmd {
	return newCli(k.lg, nil)
}

func (k kubectl) kubeVersion() string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	out := k.cli().must(ctx, k.kt.versionCmd()...)
	cancel()

	re := regexp.MustCompile(`Server Version:.*?GitVersion: *"([^"]+)"`)
	version := re.FindStringSubmatch(out)
	if len(version) != 2 {
		k.lg.Fatalf("Unable to extract kubernetes cluster version from kubectl version output: %s", out)
	}
	return version[1]
}

func (k kubectl) create(namespace string, args ...string) error {
	_, err := k.cli().run(context.Background(),
		append(k.kt.createCmd(namespace), args...)...)
	return err
}

func (k kubectl) delete(namespace string, args ...string) error {
	_, err := k.cli().run(context.Background(),
		append(k.kt.deleteCmd(namespace), args...)...)
	return err
}
