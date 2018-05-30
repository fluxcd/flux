package test

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"time"
)

const (
	helmVersion                 = "v2.9.1"
	tillerContactTimeoutSeconds = 5
	tillerContactTimeout        = tillerContactTimeoutSeconds * time.Second
	// Helm itself doesn't usually need much time to deploy, but if the cluster just
	// came up that may change things.
	tillerInitTimeout = 120 * time.Second
)

type (
	helmTool struct {
		profile  string
		helmhome string
	}

	helmAPI interface {
		tillerVersion() (string, error)
		delete(releaseName string, purge bool) error
		history(releaseName string) ([]helmHistory, error)
		mustGetValues(releaseName string, revision int) string
		mustUpgrade(releaseName string, chartpath string, reuseValues bool,
			valueSettings ...string)
		mustInstall(namespace string, releaseName string, chartpath string,
			valueSettings ...string)
	}

	helm struct {
		ht helmTool
		lg logger
	}

	helmHistory struct {
		Chart       string `json:"chart"`
		Description string `json:"description"`
		Revision    int    `json:"revision"`
		Status      string `json:"status"`
		Updated     string `json:"updated"`
	}
)

func (ht helmTool) common() []string {
	return []string{"helm", "--kube-context", ht.profile,
		"--home", ht.helmhome}
}

func (ht helmTool) initCmd() []string {
	return append(ht.common(),
		[]string{"init", "--wait", "--skip-refresh", "--upgrade", "--service-account", "tiller"}...)
}

func (ht helmTool) commonPostInit() []string {
	return append(ht.common(),
		[]string{"--tiller-connection-timeout",
			fmt.Sprintf("%d", tillerContactTimeoutSeconds)}...)
}

func (ht helmTool) versionCmd(clientOrServer string) []string {
	return append(ht.commonPostInit(), "version", "--"+clientOrServer)
}

func (ht helmTool) deleteCmd(releaseName string, purge bool) []string {
	delArgs := []string{"delete"}
	if purge {
		delArgs = append(delArgs, "--purge")
	}
	delArgs = append(delArgs, releaseName)
	return append(ht.commonPostInit(), delArgs...)
}

func (ht helmTool) upgradeCmd(
	releaseName string,
	chartpath string,
	reuseValues bool,
	valueSettings ...string) []string {

	upgradeArgs := []string{"upgrade", releaseName, chartpath}
	if reuseValues {
		upgradeArgs = append(upgradeArgs, "--reuse-values")
	}
	for _, v := range valueSettings {
		upgradeArgs = append(upgradeArgs, []string{"--set", v}...)
	}
	return append(ht.commonPostInit(), upgradeArgs...)
}

func (ht helmTool) installCmd(
	namespace string,
	releaseName string,
	chartpath string,
	valueSettings ...string) []string {

	installArgs := []string{"install", "--namespace", namespace, "--name", releaseName}
	for _, v := range valueSettings {
		installArgs = append(installArgs, []string{"--set", v}...)
	}
	return append(ht.commonPostInit(), append(installArgs, chartpath)...)
}

func (ht helmTool) historyCmd(releaseName string) []string {
	return append(ht.commonPostInit(), []string{"history", "-ojson", releaseName}...)
}

func (ht helmTool) getValuesCmd(releaseName string, revision int) []string {
	return append(ht.commonPostInit(), []string{"get", "values", releaseName,
		"--revision", fmt.Sprintf("%d", revision)}...)
}

func newHelmTool(profile string, helmhome string) (*helmTool, error) {
	return &helmTool{profile: profile, helmhome: helmhome}, nil
}

func mustNewHelm(lg logger, profile string, helmhome string, k kubectlAPI) helm {
	ht, err := newHelmTool(profile, helmhome)
	if err != nil {
		lg.Fatalf("%v", err)
	}

	h := helm{ht: *ht, lg: lg}
	out := h.cli().must(context.Background(), h.ht.versionCmd("client")...)
	clientVersion, err := parseHelmVersionString(out)
	if err != nil || clientVersion != helmVersion {
		lg.Fatalf("helm client version is %v but we require %v", out, helmVersion)
	}

	tillerVersion, err := h.tillerVersion()
	if err != nil || tillerVersion != helmVersion {
		h.mustInit(k)
	}

	tillerVersion, err = h.tillerVersion()
	if err != nil || tillerVersion != helmVersion {
		lg.Fatalf("running tiller version is %s but only %s is supported",
			tillerVersion, helmVersion)
	}
	return h
}

func (h helm) cli() clicmd {
	return newCli(h.lg, nil)
}

func parseHelmVersionString(s string) (string, error) {
	re := regexp.MustCompile(`Version{SemVer: *"([^"]+)"`)
	version := re.FindStringSubmatch(s)
	if len(version) != 2 {
		return "", fmt.Errorf("Error extracting tiller version from helm version output: %s", s)
	}
	return version[1], nil
}

func (h helm) tillerVersion() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), tillerContactTimeout)
	out, err := h.cli().run(ctx, h.ht.versionCmd("server")...)
	cancel()
	if err != nil {
		return "", err
	}
	return parseHelmVersionString(out)
}

func (h helm) mustInit(k kubectlAPI) {
	if err := k.create("kube-system", "sa", "tiller"); err != nil {
		h.lg.Fatalf("Unable to create tiller serviceaccount: %v", err)
	}
	err := k.create("kube-system", "clusterrolebinding", "tiller-cluster-rule",
		"--clusterrole=cluster-admin", "--serviceaccount=kube-system:tiller")
	if err != nil {
		h.lg.Fatalf("Unable to create tiller clusterrolebinding: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), tillerInitTimeout)
	h.cli().must(ctx, h.ht.initCmd()...)
	cancel()
}

func (h helm) delete(releaseName string, purge bool) error {
	_, err := h.cli().run(context.Background(), h.ht.deleteCmd(releaseName, purge)...)
	return err
}

func (h helm) history(releaseName string) ([]helmHistory, error) {
	out, err := h.cli().run(context.Background(), h.ht.historyCmd(releaseName)...)
	if err != nil {
		return nil, err
	}

	var hist []helmHistory
	if err := json.Unmarshal([]byte(out), &hist); err != nil {
		h.lg.Fatalf("Unable to parse helm history (error=%v): %q", err, out)
	}

	return hist, nil
}

func (h helm) mustGetValues(releaseName string, revision int) string {
	return h.cli().must(context.Background(), h.ht.getValuesCmd(releaseName, revision)...)
}

func (h helm) mustUpgrade(releaseName string, chartpath string, reuseValues bool, valueSettings ...string) {
	h.cli().must(context.Background(),
		h.ht.upgradeCmd(releaseName, chartpath, reuseValues, valueSettings...)...)
}

func (h helm) mustInstall(namespace string, releaseName string, chartpath string, valueSettings ...string) {
	h.cli().must(context.Background(),
		h.ht.installCmd(namespace, releaseName, chartpath, valueSettings...)...)
}
