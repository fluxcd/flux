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

// ComparePaths ... used to establish which Chart has changed since last sync
func (chs *ChartChangeSync) chartChanged(ctx context.Context, path1, path2 string) (bool, error) {
	chs.release.Repo.ChartsSync.Curr.Lock()
	defer chs.release.Repo.ChartsSync.Curr.Unlock()
	chs.release.Repo.ChartsSync.New.Lock()
	defer chs.release.Repo.ChartsSync.New.Unlock()

	if len(path1) == 0 || path1[0] != '/' {
		return false, fmt.Errorf("path [%s] must provided and must be absolute", path1)
	}
	if len(path2) == 0 || path2[0] != '/' {
		return false, fmt.Errorf("path [%s] must provided and must be absolute", path2)
	}

	out := &bytes.Buffer{}
	if err := chs.execDiffCmd(ctx, out, path1, path2); err != nil {
		return false, err
	}

	lines := splitList(out.String())
	if len(lines) < 1 {
		return false, nil
	}
	return true, nil
}

func splitList(s string) []string {
	outStr := strings.TrimSpace(s)
	if outStr == "" {
		return []string{}
	}

	lines := []string{}
	lines = strings.Split(outStr, "\n")

	return lines
}

func (chs *ChartChangeSync) execDiffCmd(ctx context.Context, out io.Writer, args ...string) error {
	args = append([]string{"-r"}, args...)
	c := exec.CommandContext(ctx, "diff", args...)

	chs.logger.Log("info", fmt.Sprintf("Running command: diff %v", args))

	c.Stdout = ioutil.Discard
	if out != nil {
		c.Stdout = out
	}
	errOut := &bytes.Buffer{}
	c.Stderr = errOut

	err := c.Run()
	if err != nil {

		// TODO/DEBUG: investigate and fix
		// Getting an &exec.ExitError when there is a diff result
		// 	This works with ls command and its output. Comparing both outputs,
		//	I can see diff output is more complicated.
		if exitError, ok := err.(*exec.ExitError); ok {
			chs.logger.Log("error", fmt.Sprintf("Failure while diffing: %#v", exitError.Error()))
			return err
		}
	}

	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("running diff command: %s %v", "diff", args)
	} else if ctx.Err() == context.Canceled {
		return fmt.Errorf("context was unexpectedly cancelled when running command: %s %v", "diff", args)
	}

	//fmt.Printf("* error while diffing = %+v\n\n", err)
	return err
}
