package chartsync

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os/exec"
	"strings"
)

func (chs *ChartChangeSync) chartChanged(ctx context.Context, dir, revRange, chart string) (bool, error) {
	chs.release.Repo.ConfigSync.Lock()
	defer chs.release.Repo.ConfigSync.Unlock()

	if len(dir) == 0 || dir[0] != '/' {
		return false, fmt.Errorf("directory must provided and must be absolute: [%s]", dir)
	}
	if len(chart) == 0 {
		return false, fmt.Errorf("chart must provided: [%s]", chart)
	}

	out := &bytes.Buffer{}
	if err := chs.execDiffCmd(ctx, dir, out, revRange, chart); err != nil {
		return false, err
	}

	lines := splitLines(out.String())
	if len(lines) < 1 {
		return false, nil
	}
	return true, nil
}

func splitLines(s string) []string {
	outStr := strings.TrimSpace(s)
	if outStr == "" {
		return nil
	}
	return strings.Split(outStr, "\n")
}

// execDiffCmd ... find if there is a change in a particular chart
//		git diff revCurr..revNew chart
//		(runs in the /repoRoot/charts dir)
//		input:
//					dir  (/repoRoot/charts)
//					args (revCurr..revNew,  chart)
func (chs *ChartChangeSync) execDiffCmd(ctx context.Context, dir string, out io.Writer, args ...string) error {
	args = append([]string{"diff"}, args...)
	c := exec.CommandContext(ctx, "git", args...)

	chs.logger.Log("info", fmt.Sprintf("Running command: git %v", args))

	c.Dir = dir

	c.Stdout = ioutil.Discard
	if out != nil {
		c.Stdout = out
	}
	errOut := &bytes.Buffer{}
	c.Stderr = errOut

	err := c.Run()
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			chs.logger.Log("error", fmt.Sprintf("Failure while running git diff: %#v", exitError.Error()))
			return err
		}
	}

	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("running git diff command: %s %v", "git", args)
	} else if ctx.Err() == context.Canceled {
		return fmt.Errorf("context was unexpectedly cancelled when running command: %s %v", "gitt", args)
	}

	return err
}
