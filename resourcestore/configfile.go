package resourcestore

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

	"github.com/weaveworks/flux"
)

const (
	ConfigFilename = ".flux.yaml"
	CommandTimeout = time.Minute
)

type ConfigFile struct {
	Path       string
	WorkingDir string
	Version    string `yaml:"version"`
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
	Annotation     AnnotationUpdater
}

type ContainerImageUpdater struct {
	Command string
}

type AnnotationUpdater struct {
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
	case result.CommandUpdated != nil && result.PatchUpdated != nil:
		return nil, errors.New("either commandUpdated or patchUpdated should be defined")
	case result.PatchUpdated != nil && result.PatchUpdated.PatchFile == "":
		return nil, errors.New("patchUpdated's patchFile cannot be empty")
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
		// Stop exectuing on the first command error
		if err != nil {
			break
		}
	}
	return result
}

// ExecContainerImageUpdaters executes all the image updates in the configuration file.
// It will stop at the first error, in which case the returned error will be non-nil
func (cf *ConfigFile) ExecContainerImageUpdaters(ctx context.Context,
	workload flux.ResourceID, container string, image, imageTag string) []ConfigFileCombinedExecResult {
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

// ExecAnnotationUpdaters executes all the annotation updates in the configuration file.
// It will stop at the first error, in which case the returned error will be non-nil
func (cf *ConfigFile) ExecAnnotationUpdaters(ctx context.Context,
	workload flux.ResourceID, annotationKey string, annotationValue *string) []ConfigFileCombinedExecResult {
	env := makeEnvFromResourceID(workload)
	env = append(env, "FLUX_ANNOTATION_KEY="+annotationKey)
	if annotationValue != nil {
		env = append(env, "FLUX_ANNOTATION_VALUE="+*annotationValue)
	}
	commands := []string{}
	var updaters []Updater
	if cf.CommandUpdated != nil {
		updaters = cf.CommandUpdated.Updaters
	}
	for _, u := range updaters {
		commands = append(commands, u.Annotation.Command)
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

func makeEnvFromResourceID(id flux.ResourceID) []string {
	ns, kind, name := id.Components()
	return []string{
		"FLUX_WORKLOAD=" + id.String(),
		"FLUX_WL_NS=" + ns,
		"FLUX_WL_KIND=" + kind,
		"FLUX_WL_NAME=" + name,
	}
}
