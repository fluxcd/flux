// +build integration_test

package test

import (
	"context"
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	defaultHelloworldPort = 30030
	defaultSidecarPort    = 30031
	releaseName1          = "test1"
	defaultPollInterval   = 5 * time.Second
	yq                    = "bin/yq"
)

func (h *harness) installFluxChart(pollinterval time.Duration) {
	h.helmAPI.delete(helmFluxRelease, true)
	// Hack until #1009 is fixed.
	h.helmAPI.delete(releaseName1, true)
	h.helmAPI.mustInstall(fluxNamespace, helmFluxRelease, "helm/charts/weave-flux",
		"helmOperator.create=true",
		"git.url="+h.gitURL(),
		"git.chartsPath=charts",
		"image.tag=latest",
		"helmOperator.tag=latest",
		"git.pollInterval="+pollinterval.String())
}

func (h *harness) gitAddCommitPushSync() {
	ctx, cancel := context.WithTimeout(context.Background(), syncTimeout)
	h.mustAddCommitPush()
	h.waitForSync(ctx, "HEAD")
	cancel()
}

func (h *harness) pushNewHelmFluxRepo(ctx context.Context) {
	execNoErr(ctx, h.t, "cp", "-rT", "helm/repo", h.repodir)
	h.gitAddCommitPushSync()
}

func (h *harness) initHelmTest(pollinterval time.Duration) {
	h.installFluxChart(pollinterval)
	h.pushNewHelmFluxRepo(context.Background())
}

func (h *harness) lastHelmRelease(releaseName string) (helmHistory, error) {
	// There may be one or two history entries, depending on timing.  It
	// seems there's an unnecessary upgrade happening, but only once.
	hist, err := h.helmAPI.history(releaseName)
	if err != nil {
		return helmHistory{}, err
	}
	if len(hist) == 0 {
		return helmHistory{}, fmt.Errorf("error getting last helm history entry, none found")
	}
	return hist[len(hist)-1], nil
}

func (h *harness) helmReleaseDeployed(hist helmHistory, releaseName string, minRevision int) error {
	if hist.Revision < minRevision {
		return fmt.Errorf("helm release revision of %q is %d, smaller than our min of %d", releaseName, hist.Revision, minRevision)
	}
	if hist.Status != "DEPLOYED" {
		return fmt.Errorf("helm release status of %q is %q rather than DEPLOYED", releaseName, hist.Status)
	}
	return nil
}

func (h *harness) helmReleaseHasValue(releaseName string, minRevision int, key, val string) error {
	hist, err := h.lastHelmRelease(releaseName)
	if err != nil {
		return err
	}
	if err := h.helmReleaseDeployed(hist, releaseName, minRevision); err != nil {
		return err
	}

	valstr := h.helmAPI.mustGetValues(releaseName, hist.Revision)
	yqout := strings.TrimSpace(strOrDie(
		envExecStdin(context.Background(), h.t, nil, strings.NewReader(valstr), yq, "r", "-", key)))
	if val != yqout {
		return fmt.Errorf("expected value for %q is %q, got %q", key, val, yqout)
	}
	return nil
}

func (h *harness) assertHelmReleaseDeployed(releaseName string, minRevision int) int {
	var hist helmHistory
	ctx, cancel := context.WithTimeout(context.Background(), releaseTimeout)
	defer cancel()
	h.must(until(ctx, func(ictx context.Context) error {
		var err error
		hist, err = h.lastHelmRelease(releaseName)
		if err != nil {
			return err
		}
		return h.helmReleaseDeployed(hist, releaseName, minRevision)
	}))
	return hist.Revision
}

func (h *harness) assertHelmReleaseHasValue(timeout time.Duration, releaseName string, minRevision int, key, val string) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	h.must(until(ctx, func(ictx context.Context) error {
		return h.helmReleaseHasValue(releaseName, minRevision, key, val)
	}))
}

func (h *harness) updateGitYaml(relpath string, yamlpath string, value string) {
	execNoErr(context.Background(), h.t, yq, "w", "-i",
		filepath.Join(h.repodir, relpath), yamlpath, value)
}

func TestChart(t *testing.T) {
	h := newharness(t)
	h.initHelmTest(defaultPollInterval)

	h.assertHelmReleaseDeployed(releaseName1, 1)

	h.must(httpGetReturns(h.clusterIP, defaultHelloworldPort, "Ahoy\n"))
	h.must(httpGetReturns(h.clusterIP, defaultSidecarPort, "I am a sidecar\n"))
}

func TestChartUpdateViaGit(t *testing.T) {
	h := newharness(t)
	h.initHelmTest(defaultPollInterval)

	initialRevision := h.assertHelmReleaseDeployed(releaseName1, 1)
	h.must(httpGetReturns(h.clusterIP, defaultHelloworldPort, "Ahoy\n"))
	h.must(httpGetReturns(h.clusterIP, defaultSidecarPort, "I am a sidecar\n"))

	// obviously this should work if the above works, it's just to
	// contrast with the Dial invocation below
	_, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", h.clusterIP, defaultSidecarPort), 5*time.Second)
	h.must(err)

	newMessage := "salut"
	newSidecarPort := defaultSidecarPort + 2
	h.updateGitYaml("releases/helloworld.yaml", "spec.values.hellomessage", newMessage)
	h.updateGitYaml("releases/helloworld.yaml", "spec.values.service.sidecar.port",
		fmt.Sprintf("%d", newSidecarPort))
	h.gitAddCommitPushSync()

	h.assertHelmReleaseDeployed(releaseName1, initialRevision+1)
	h.must(httpGetReturns(h.clusterIP, defaultHelloworldPort, newMessage+"\n"))
	h.must(httpGetReturns(h.clusterIP, newSidecarPort, "I am a sidecar\n"))

	_, err = net.DialTimeout("tcp", fmt.Sprintf("%s:%d", h.clusterIP, defaultSidecarPort), 5*time.Second)
	if err == nil {
		t.Errorf("old sidecar port %d still open", defaultSidecarPort)
	}
}

func TestChartUpdateViaHelm(t *testing.T) {
	h := newharness(t)
	pollInterval := 20 * time.Second
	h.initHelmTest(pollInterval)

	initialRevision := h.assertHelmReleaseDeployed(releaseName1, 1)
	h.must(httpGetReturns(h.clusterIP, defaultHelloworldPort, "Ahoy\n"))
	h.must(httpGetReturns(h.clusterIP, defaultSidecarPort, "I am a sidecar\n"))

	key, val := "hellomessage", "greetings"
	h.helmAPI.mustUpgrade(releaseName1,
		filepath.Join(h.repodir, "charts", "helloworld"),
		true, fmt.Sprintf("%s=%s", key, val))

	h.assertHelmReleaseHasValue(releaseTimeout, releaseName1, initialRevision+1, key, val)
	h.must(httpGetReturns(h.clusterIP, defaultHelloworldPort, val+"\n"))

	// TODO specify minrevision more precisely
	h.assertHelmReleaseHasValue(releaseTimeout+pollInterval, releaseName1, initialRevision+1, key, "null")
	h.must(httpGetReturns(h.clusterIP, defaultHelloworldPort, "Ahoy\n"))
}

// TODO tests:
// - chart with README template
// - deploy chart directly via helm, verify not touched by flux
