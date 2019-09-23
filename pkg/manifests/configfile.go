package manifests

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"time"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"

	"github.com/fluxcd/flux/pkg/resource"
)

const (
	ConfigFilename = ".flux.yaml"
	CommandTimeout = time.Minute
)

type ConfigFile struct {
	Path       string
	WorkingDir string
	Version    int
	// Only one of the following should be set simultaneously
	CommandUpdated *CommandUpdated `yaml:"commandUpdated"`
	PatchUpdated   *PatchUpdated   `yaml:"patchUpdated"`
}

type CommandUpdated struct {
	Generators []Generator
	Updaters   []Updater
}

type Generator struct {
	Command string
}

type Updater struct {
	ContainerImage ContainerImageUpdater `yaml:"containerImage"`
	Policy         PolicyUpdater
}

type ContainerImageUpdater struct {
	Command string
}

type PolicyUpdater struct {
	Command string
}

type PatchUpdated struct {
	Generators []Generator
	PatchFile  string `yaml:"patchFile"`
}

func NewConfigFile(path, workingDir string) (*ConfigFile, error) {
	var result ConfigFile
	fileBytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read: %s", err)
	}
	if err := yaml.Unmarshal(fileBytes, &result); err != nil {
		return nil, fmt.Errorf("cannot parse: %s", err)
	}
	result.Path = path
	result.WorkingDir = workingDir
	switch {
	case (result.CommandUpdated != nil && result.PatchUpdated != nil) ||
		(result.CommandUpdated == nil && result.PatchUpdated == nil):
		return nil, errors.New("a single commandUpdated or patchUpdated entry must be defined")
	case result.PatchUpdated != nil && result.PatchUpdated.PatchFile == "":
		return nil, errors.New("patchUpdated's patchFile cannot be empty")
	case result.Version != 1:
		return nil, errors.New("incorrect version, only version 1 is supported for now")
	}
	return &result, nil
}

type ConfigFileExecResult struct {
	Error  error
	Stderr []byte
	Stdout []byte
}

type ConfigFileCombinedExecResult struct {
	Error  error
	Output []byte
}

func (cf *ConfigFile) ExecGenerators(ctx context.Context, generators []Generator) []ConfigFileExecResult {
	result := []ConfigFileExecResult{}
	for _, g := range generators {
		stdErr := bytes.NewBuffer(nil)
		stdOut := bytes.NewBuffer(nil)
		err := cf.execCommand(ctx, nil, stdOut, stdErr, g.Command)
		r := ConfigFileExecResult{
			Stdout: stdOut.Bytes(),
			Stderr: stdErr.Bytes(),
			Error:  err,
		}
		result = append(result, r)
		// Stop executing on the first command error
		if err != nil {
			break
		}
	}
	return result
}

// ExecContainerImageUpdaters executes all the image updates in the configuration file.
// It will stop at the first error, in which case the returned error will be non-nil
func (cf *ConfigFile) ExecContainerImageUpdaters(ctx context.Context,
	workload resource.ID, container string, image, imageTag string) []ConfigFileCombinedExecResult {
	env := makeEnvFromResourceID(workload)
	env = append(env,
		"FLUX_CONTAINER="+container,
		"FLUX_IMG="+image,
		"FLUX_TAG="+imageTag,
	)
	commands := []string{}
	var updaters []Updater
	if cf.CommandUpdated != nil {
		updaters = cf.CommandUpdated.Updaters
	}
	for _, u := range updaters {
		commands = append(commands, u.ContainerImage.Command)
	}
	return cf.execCommandsWithCombinedOutput(ctx, env, commands)
}

// ExecPolicyUpdaters executes all the policy update commands given in
// the configuration file. An empty policyValue means remove the
// policy. It will stop at the first error, in which case the returned
// error will be non-nil
func (cf *ConfigFile) ExecPolicyUpdaters(ctx context.Context,
	workload resource.ID, policyName, policyValue string) []ConfigFileCombinedExecResult {
	env := makeEnvFromResourceID(workload)
	env = append(env, "FLUX_POLICY="+policyName)
	if policyValue != "" {
		env = append(env, "FLUX_POLICY_VALUE="+policyValue)
	}
	commands := []string{}
	var updaters []Updater
	if cf.CommandUpdated != nil {
		updaters = cf.CommandUpdated.Updaters
	}
	for _, u := range updaters {
		commands = append(commands, u.Policy.Command)
	}
	return cf.execCommandsWithCombinedOutput(ctx, env, commands)
}

func (cf *ConfigFile) execCommandsWithCombinedOutput(ctx context.Context, env []string, commands []string) []ConfigFileCombinedExecResult {
	env = append(env, "PATH="+os.Getenv("PATH"))
	result := []ConfigFileCombinedExecResult{}
	for _, c := range commands {
		stdOutAndErr := bytes.NewBuffer(nil)
		err := cf.execCommand(ctx, env, stdOutAndErr, stdOutAndErr, c)
		r := ConfigFileCombinedExecResult{
			Output: stdOutAndErr.Bytes(),
			Error:  err,
		}
		result = append(result, r)
		// Stop executing on the first command error
		if err != nil {
			break
		}
	}
	return result
}

func (cf *ConfigFile) execCommand(ctx context.Context, env []string, stdOut, stdErr io.Writer, command string) error {
	cmdCtx, cancel := context.WithTimeout(ctx, CommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", command)
	cmd.Env = env
	cmd.Dir = cf.WorkingDir
	cmd.Stdout = stdOut
	cmd.Stderr = stdErr
	err := cmd.Run()
	if cmdCtx.Err() == context.DeadlineExceeded {
		err = cmdCtx.Err()
	} else if cmdCtx.Err() == context.Canceled {
		err = errors.Wrap(ctx.Err(), fmt.Sprintf("context was unexpectedly cancelled"))
	}
	return err
}

func makeEnvFromResourceID(id resource.ID) []string {
	ns, kind, name := id.Components()
	return []string{
		"FLUX_WORKLOAD=" + id.String(),
		"FLUX_WL_NS=" + ns,
		"FLUX_WL_KIND=" + kind,
		"FLUX_WL_NAME=" + name,
	}
}
