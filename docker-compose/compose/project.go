package compose

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"

	"github.com/aoldershaw/prototype-experiments/docker-compose/docker"
	"github.com/aoldershaw/prototype-sdk-go"
)

type Project struct {
	Files     OneOrMany         `json:"file" prototype:"required"`
	Name      string            `json:"project_name,omitempty"`
	Directory string            `json:"project_directory,omitempty"`
	Profiles  OneOrMany         `json:"profile,omitempty"`
	Images    []string          `json:"images,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	EnvFile   string            `json:"envfile,omitempty"`
}

type UpParams struct {
	Build                bool           `json:"build,omitempty"`
	NoBuild              bool           `json:"no_build,omitempty"`
	AbortOnContainerExit bool           `json:"abort_on_container_exit,omitempty"`
	ExitCodeFrom         string         `json:"exit_code_from,omitempty"`
	Timeout              int            `json:"timeout,omitempty"`
	NoLogPrefix          bool           `json:"no_log_prefix,omitempty"`
	Scale                map[string]int `json:"scale,omitempty"`
}

func (project Project) Up(params UpParams) ([]prototype.MessageResponse, error) {
	if err := docker.Start(); err != nil {
		return nil, fmt.Errorf("failed to start docker: %w", err)
	}
	defer docker.Stop()

	if err := loadImages(project); err != nil {
		return nil, fmt.Errorf("failed to load images: %w", err)
	}

	cmd := exec.Command("docker-compose")
	cmd.Args = append(cmd.Args, project.Flags()...)
	cmd.Args = append(cmd.Args, "up")
	cmd.Args = append(cmd.Args, params.Flags()...)
	cmd.Env = project.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("up: %w", err)
	}

	return nil, nil
}

func loadImages(project Project) error {
	for _, img := range project.Images {
		err := docker.Load(img)
		if err != nil {
			return fmt.Errorf("image %s: %w", img, err)
		}
	}
	return nil
}

func (project Project) Flags() []string {
	var flags []string
	for _, file := range project.Files {
		flags = append(flags, "-f", file)
	}
	if project.Name != "" {
		flags = append(flags, "-p", project.Name)
	}
	if project.Directory != "" {
		flags = append(flags, "--project-directory", project.Directory)
	}
	for _, profile := range project.Profiles {
		flags = append(flags, "--profile", profile)
	}
	if project.EnvFile != "" {
		flags = append(flags, "--env-file", project.EnvFile)
	}
	return flags
}

func (project Project) Environ() []string {
	env := make([]string, 0, len(project.Env))
	for name, val := range project.Env {
		env = append(env, fmt.Sprintf("%s=%s", name, val))
	}
	return env
}

func (params UpParams) Flags() []string {
	var flags []string
	if params.Build {
		flags = append(flags, "--build")
	}
	if params.NoBuild {
		flags = append(flags, "--no-build")
	}
	if params.AbortOnContainerExit {
		flags = append(flags, "--abort-on-container-exit")
	}
	if params.ExitCodeFrom != "" {
		flags = append(flags, "--exit-code-from", params.ExitCodeFrom)
	}
	if params.Timeout != 0 {
		flags = append(flags, "--timeout", strconv.Itoa(params.Timeout))
	}
	if params.NoLogPrefix {
		flags = append(flags, "--no-log-prefix")
	}
	for service, scale := range params.Scale {
		flags = append(flags, "--scale", fmt.Sprintf("%s=%d", service, scale))
	}
	return flags
}
