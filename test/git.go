package test

import (
	"context"
	"fmt"
	"os"
	"time"
)

type (
	gitTool struct {
		repodir string
	}

	git struct {
		gitSSHCommand string
		gitOriginURL  string
		gt            gitTool
		lg            logger
	}

	gitAPI interface {
		mustFetch()
		mustAddCommitPush()
		revlist(args ...string) (string, error)
	}
)

func (gt gitTool) common() []string {
	return []string{"git", "-C", gt.repodir}
}

func (gt gitTool) initCmd() []string {
	return []string{"git", "init", gt.repodir}
}

func (gt gitTool) cloneCmd(originURL string) []string {
	return []string{"git", "clone", originURL, gt.repodir}
}

func (gt gitTool) addCmd(files ...string) []string {
	return append(gt.common(),
		append([]string{"add"}, files...)...)
}

func (gt gitTool) commitCmd(msg string) []string {
	return append(gt.common(), []string{"commit", "-m", msg}...)
}

func (gt gitTool) pushCmd() []string {
	return append(gt.common(), []string{"push", "-u", "origin", "master"}...)
}

func (gt gitTool) revlistCmd(args ...string) []string {
	return append(gt.common(),
		append([]string{"rev-list"}, args...)...)
}

func (gt gitTool) fetchCmd() []string {
	return append(gt.common(), []string{"fetch", "--tags"}...)
}

func newGitTool(repodir string) (*gitTool, error) {
	_, err := os.Stat(repodir)
	if err == nil || !os.IsNotExist(err) {
		return nil, fmt.Errorf("git repodir %s must not already exist", repodir)
	}
	return &gitTool{repodir: repodir}, nil
}

func mustNewGit(lg logger, repodir string, sshcmd string, origin string) git {
	gt, err := newGitTool(repodir)
	if err != nil {
		lg.Fatalf("%v", err)
	}

	g := git{gt: *gt, lg: lg, gitSSHCommand: sshcmd, gitOriginURL: origin}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	g.cli().must(ctx, gt.cloneCmd(origin)...)
	cancel()

	return g
}

func (g git) cli() clicmd {
	return newCli(g.lg, []string{"GIT_SSH_COMMAND=" + g.gitSSHCommand})
}

func (g git) mustFetch() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	g.cli().must(ctx, g.gt.fetchCmd()...)
	cancel()
}

func (g git) mustAddCommitPush() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	g.cli().must(ctx, g.gt.addCmd(".")...)
	g.cli().must(ctx, g.gt.commitCmd("deploy")...)
	g.cli().must(ctx, g.gt.pushCmd()...)
	cancel()
}

func (g git) revlist(args ...string) (string, error) {
	return g.cli().run(context.Background(), g.gt.revlistCmd(args...)...)
}
