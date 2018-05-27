// +build integration_test

package test

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/weaveworks/flux/api/v6"
	transport "github.com/weaveworks/flux/http"
	"github.com/weaveworks/flux/http/client"
	"github.com/weaveworks/flux/image"
)

func strOrDie(s string, err error) string {
	if err != nil {
		log.Fatal(err)
	}
	return s
}

func ignoreErr(s string, err error) string {
	if err != nil {
		return ""
	}
	return s
}

func envExec(ctx context.Context, t *testing.T, env []string, command string, args ...string) (string, error) {
	return envExecStdin(ctx, t, env, strings.NewReader(""), command, args...)
}

func envExecStdin(ctx context.Context, t *testing.T, env []string, stdin io.Reader, command string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Env = env
	cmd.Stdin = stdin
	if t != nil {
		t.Logf("running %v", cmd.Args)
	} else {
		log.Printf("running %v", cmd.Args)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		err = fmt.Errorf("error running %v: %v\nOutput:\n%s", cmd.Args, err, out)
	}
	return string(out), err
}

func envExecNoErr(ctx context.Context, t *testing.T, env []string, command string, args ...string) string {
	out, err := envExec(ctx, t, env, command, args...)
	return strOrDie(string(out), err)
}

func execNoErr(ctx context.Context, t *testing.T, command string, args ...string) string {
	return envExecNoErr(ctx, t, nil, command, args...)
}

func until(ctx context.Context, f func(context.Context) error) error {
	var err error
	ticker := time.NewTicker(time.Second)
	for {
		select {
		case <-ticker.C:
			err = f(ctx)
			if err == nil {
				return nil
			}
		case <-ctx.Done():
			return fmt.Errorf("timed out, last error: %v", err)
		}
	}
}

func fluxServicesAPICall(ctx context.Context, fluxURL string, namespace string) ([]v6.ControllerStatus, error) {
	api := client.New(http.DefaultClient, transport.NewAPIRouter(), fluxURL, "")
	var controllers []v6.ControllerStatus
	return controllers, until(ctx, func(ictx context.Context) error {
		var err error
		controllers, err = api.ListServices(ictx, namespace)
		return err
	})
}

// fluxServices asks flux for the services it's managing, return a map from container name to id.
func fluxServices(ctx context.Context, fluxURL string, t *testing.T, namespace string, id string) map[string]image.Ref {
	controllers, err := fluxServicesAPICall(ctx, fluxURL, namespace)
	if err != nil {
		t.Errorf("failed to fetch controllers from flux agent: %v", err)
	}

	result := make(map[string]image.Ref)
	for _, controller := range controllers {
		if controller.ID.String() == id {
			for _, c := range controller.Containers {
				result[c.Name] = c.Current.ID
			}
		}
	}

	return result
}

func httpGet(ctx context.Context, url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	return string(body), err
}

func httpGetReturns(host string, port int, expected string) error {
	url := fmt.Sprintf("http://%s:%d", host, port)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return until(ctx, func(ictx context.Context) error {
		got, err := httpGet(ictx, url)
		if err != nil || got != expected {
			return fmt.Errorf("service check on %d failed, got %q, error: %v", port, got, err)
		}
		return nil
	})
}
