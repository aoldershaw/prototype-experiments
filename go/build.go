package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"text/template"

	"github.com/aoldershaw/prototype-sdk-go"
)

const DefaultOutputTemplate = "{{.Dir}}-{{.OS}}-{{.Arch}}"

type BuildParams struct {
	Package OneOrMany `json:"package"`
	OS      OneOrMany `json:"os"`
	Arch    OneOrMany `json:"arch"`

	SkipPlatforms []Platform `json:"skip_platforms"`

	OutputTemplate string `json:"output_template"`

	Ldflags         string              `json:"ldflags"`
	PlatformLdflags map[Platform]string `json:"platform_ldflags"`

	Gcflags         string              `json:"gcflags"`
	PlatformGcflags map[Platform]string `json:"platform_gcflags"`

	Asmflags         string              `json:"asmflags"`
	PlatformAsmflags map[Platform]string `json:"platform_asmflags"`

	Tags    []string `json:"tags"`
	ModMode string   `json:"mod"`
	Rebuild bool     `json:"rebuild"`
	Race    bool     `json:"race"`
	Cgo     bool     `json:"cgo"`

	Parallelism int `json:"parallelism"`
}

// Platforms returns the list of platforms defined in the build matrix. It
// includes those platforms that were marked as skipped in SkipPlatforms, which
// should be filtered out elsewhere.
func (p BuildParams) Platforms() []Platform {
	if len(p.OS) == 0 {
		p.OS = OneOrMany{runtime.GOOS}
	}
	if len(p.Arch) == 0 {
		p.Arch = OneOrMany{runtime.GOARCH}
	}
	var platforms []Platform
	for _, os := range p.OS {
		for _, arch := range p.Arch {
			platforms = append(platforms, Platform{OS: os, Arch: arch})
		}
	}
	return platforms
}

type OutputTemplateParams struct {
	Dir  string
	OS   string
	Arch string
}

type BuildID struct {
	Platform Platform
	Package  string
}

type BuildOptions struct {
	BuildID
	Output   string
	Ldflags  string
	Gcflags  string
	Asmflags string
	Tags     []string
	ModMode  string
	Rebuild  bool
	Race     bool
	Cgo      bool
}

func (m Module) Build(params BuildParams) ([]prototype.MessageResponse, error) {
	outputDirRel := "./output"
	outputDirAbs, err := filepath.Abs(outputDirRel)
	if err != nil {
		return nil, fmt.Errorf("determine absolute path to output dir: %w", err)
	}
	err = os.MkdirAll(outputDirAbs, 0755)
	if err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	if params.OutputTemplate == "" {
		params.OutputTemplate = DefaultOutputTemplate
	}
	outputTemplate, err := template.New("output").Parse(params.OutputTemplate)
	if err != nil {
		return nil, fmt.Errorf("invalid output template: %w", err)
	}

	if len(params.Package) == 0 {
		params.Package = OneOrMany{"."}
	}
	mainPkgs, err := m.locateMainPackages(params.Package...)
	if err != nil {
		return nil, fmt.Errorf("failed to locate packages: %w", err)
	}

	platforms := params.Platforms()

	parallelism := 1
	if params.Parallelism > 0 {
		parallelism = params.Parallelism
	}
	numBuildsTotal := len(mainPkgs) * len(platforms)
	if parallelism > numBuildsTotal {
		parallelism = numBuildsTotal
	}
	fmt.Printf("running %d build(s) in parallel...\n\n", parallelism)
	semaphore := make(chan struct{}, parallelism)

	statusCh := make(chan StatusUpdate, 1)
	ui := &UI{BuildStates: make(map[BuildID]BuildState)}

	uiDone := make(chan struct{})
	go func() {
		for update := range statusCh {
			ui.Update(update)
		}
		close(uiDone)
	}()

	var wg sync.WaitGroup
	for _, pkg := range mainPkgs {
		for _, platform := range platforms {
			buildID := BuildID{Platform: platform, Package: pkg}

			if containsPlatform(params.SkipPlatforms, platform) {
				statusCh <- StatusUpdate{
					BuildID: buildID,
					Status:  "skipped",
					Data:    "included in skip_platforms",
				}
				continue
			}

			wg.Add(1)
			semaphore <- struct{}{}

			statusCh <- StatusUpdate{
				BuildID: buildID,
				Status:  "start",
			}

			valueOrOverride := func(value string, overrides map[Platform]string) string {
				if override, ok := overrides[platform]; ok {
					return override
				}
				return value
			}

			binaryName := new(bytes.Buffer)
			err := outputTemplate.Execute(binaryName, OutputTemplateParams{
				Dir:  filepath.Base(pkg),
				OS:   platform.OS,
				Arch: platform.Arch,
			})
			if err != nil {
				statusCh <- StatusUpdate{
					BuildID: buildID,
					Status:  "error",
					Data:    err.Error(),
				}
				<-semaphore
				continue
			}
			if platform.OS == "windows" {
				binaryName.WriteString(".exe")
			}
			buildOptions := BuildOptions{
				BuildID:  buildID,
				Output:   filepath.Join(outputDirAbs, binaryName.String()),
				Ldflags:  valueOrOverride(params.Ldflags, params.PlatformLdflags),
				Gcflags:  valueOrOverride(params.Gcflags, params.PlatformGcflags),
				Asmflags: valueOrOverride(params.Asmflags, params.PlatformAsmflags),
				Tags:     params.Tags,
				ModMode:  params.ModMode,
				Rebuild:  params.Rebuild,
				Race:     params.Race,
				Cgo:      params.Cgo,
			}
			go func() {
				statusCh <- m.buildSingle(buildOptions)

				<-semaphore
				wg.Done()
			}()
		}
	}

	wg.Wait()
	close(statusCh)
	<-uiDone

	ui.PrintResult()

	return []prototype.MessageResponse{{
		Object: map[string]interface{}{
			"built": prototype.Artifact(outputDirRel),
		},
	}}, nil
}

func (m Module) buildSingle(opts BuildOptions) StatusUpdate {
	cmd := exec.Command("go", "build", "-o", opts.Output)
	if opts.Rebuild {
		cmd.Args = append(cmd.Args, "-a")
	}
	if opts.ModMode != "" {
		cmd.Args = append(cmd.Args, "-mod", opts.ModMode)
	}
	if opts.Race {
		cmd.Args = append(cmd.Args, "-race")
	}
	if len(opts.Tags) > 0 {
		cmd.Args = append(cmd.Args, "-tags", strings.Join(opts.Tags, ","))
	}
	if opts.Ldflags != "" {
		cmd.Args = append(cmd.Args, "-ldflags", opts.Ldflags)
	}
	if opts.Gcflags != "" {
		cmd.Args = append(cmd.Args, "-gcflags", opts.Gcflags)
	}
	if opts.Asmflags != "" {
		cmd.Args = append(cmd.Args, "-asmflags", opts.Asmflags)
	}
	cmd.Args = append(cmd.Args, opts.Package)

	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env,
		"GOOS="+opts.Platform.OS,
		"GOARCH="+opts.Platform.Arch,
	)
	if opts.Cgo {
		cmd.Env = append(cmd.Env, "CGO_ENABLED=1")
	} else {
		cmd.Env = append(cmd.Env, "CGO_ENABLED=0")
	}

	cmd.Dir = m.Path
	_, err := execute(cmd)
	if err != nil {
		var execErr ExecutionError
		if errors.As(err, &execErr) && strings.Contains(execErr.Stderr, "cmd/go: unsupported GOOS/GOARCH pair") {
			return StatusUpdate{
				BuildID: opts.BuildID,
				Status:  "skipped",
				Data:    "unsupported platform",
			}
		}

		return StatusUpdate{
			BuildID: opts.BuildID,
			Status:  "error",
			Data:    err.Error(),
		}
	}
	return StatusUpdate{
		BuildID: opts.BuildID,
		Status:  "success",
	}
}
