package test

import (
	"context"
	"fmt"
	"log"
	"os/exec"
)

type (
	logger interface {
		Logf(string, ...interface{})
		Errorf(string, ...interface{})
		Fatalf(string, ...interface{})
		Name() string
		Helper()
	}

	clicmd interface {
		// run executes the tool and returns the result
		run(ctx context.Context, args ...string) (string, error)
		// input executes the tool feeding it input on stdin
		input(ctx context.Context, in string, args ...string) (string, error)
		// output executes the tool and returns the result, empty on failure
		output(ctx context.Context, args ...string) string
		// must executes the tool and returns the result, dying on failure
		must(ctx context.Context, args ...string) string
	}

	cmdrunner struct {
		env []string
		lg  logger
	}

	stdLogger struct{}
)

func (l stdLogger) Name() string {
	return ""
}

func (l stdLogger) Fatalf(s string, args ...interface{}) {
	log.Fatalf(s, args...)
}

func (l stdLogger) Errorf(s string, args ...interface{}) {
	log.Printf("Error: "+s, args...)
}

func (l stdLogger) Logf(s string, args ...interface{}) {
	log.Printf(s, args...)
}

func (l stdLogger) Helper() {}

func newCli(lg logger, env []string) clicmd {
	return cmdrunner{lg: lg, env: env}
}

func stdCli() clicmd {
	return newCli(stdLogger{}, nil)
}

func (cr cmdrunner) run(ctx context.Context, args ...string) (string, error) {
	return cr.input(ctx, "", args...)
}

func (cr cmdrunner) input(ctx context.Context, in string, args ...string) (string, error) {
	cr.lg.Helper()
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Env = cr.env
	if cmd.Env != nil {
		cr.lg.Logf("running %v with env=%v", cmd.Args, cmd.Env)
	} else {
		cr.lg.Logf("running %v", cmd.Args)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		err = fmt.Errorf("error running %v: %v\nOutput:\n%s", cmd.Args, err, out)
	}
	return string(out), err
}

func (cr cmdrunner) output(ctx context.Context, args ...string) string {
	out, err := cr.run(ctx, args...)
	if err != nil {
		return ""
	}
	return out
}

func (cr cmdrunner) must(ctx context.Context, args ...string) string {
	out, err := cr.run(ctx, args...)
	if err != nil {
		cr.lg.Fatalf("%v", err)
	}
	return out
}
