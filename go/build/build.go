package build

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

	"github.com/aoldershaw/prototype-experiments/go/module"
	"github.com/aoldershaw/prototype-sdk-go"
)

const DefaultOutputTemplate = "{{.Dir}}-{{.OS}}-{{.Arch}}"

var DefaultPlatform = Platform{
	OS:   runtime.GOOS,
	Arch: runtime.GOARCH,
}

type Params struct {
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
func (p Params) Platforms() []Platform {
	if len(p.OS) == 0 {
		p.OS = OneOrMany{DefaultPlatform.OS}
	}
	if len(p.Arch) == 0 {
		p.Arch = OneOrMany{DefaultPlatform.Arch}
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

type ID struct {
	Platform Platform
	Package  string
}

type Options struct {
	ID
	Output   string
	Gopath   string
	Ldflags  string
	Gcflags  string
	Asmflags string
	Tags     []string
	ModMode  string
	Rebuild  bool
	Race     bool
	Cgo      bool
}

type Module interface {
	Execute(*exec.Cmd) error
	ResolvePackages(packages ...string) ([]module.Package, error)
}

func Build(mod Module, params Params) ([]prototype.MessageResponse, error) {
	outputDir := "./output"
	err := os.MkdirAll(outputDir, 0755)
	if err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	gopathDir := "./gopath"
	err = os.MkdirAll(gopathDir, 0755)
	if err != nil {
		return nil, fmt.Errorf("failed to create gopath directory: %w", err)
	}

	ui := NewUI()
	uiDone := make(chan struct{})
	statusCh := make(chan Status, 1)

	go func() {
		for update := range statusCh {
			ui.Update(update)
		}
		close(uiDone)
	}()

	if err := build(mod, params, outputDir, gopathDir, statusCh); err != nil {
		return nil, err
	}

	close(statusCh)
	<-uiDone

	ui.PrintResult()

	return []prototype.MessageResponse{{
		Object: map[string]interface{}{
			"built":  prototype.Artifact(outputDir),
			"gopath": prototype.Artifact(gopathDir),
		},
	}}, nil
}

func build(mod Module, params Params, outputDir, gopathDir string, statusCh chan<- Status) error {
	// get absolute paths since go command runs in a different directory
	outputDir, err := filepath.Abs(outputDir)
	if err != nil {
		return fmt.Errorf("get absolute path: %w", err)
	}
	gopathDir, err = filepath.Abs(gopathDir)
	if err != nil {
		return fmt.Errorf("get absolute path: %w", err)
	}

	if params.OutputTemplate == "" {
		params.OutputTemplate = DefaultOutputTemplate
	}
	outputTemplate, err := template.New("output").Parse(params.OutputTemplate)
	if err != nil {
		return fmt.Errorf("invalid output template: %w", err)
	}

	if len(params.Package) == 0 {
		params.Package = OneOrMany{"."}
	}
	packages, err := mod.ResolvePackages(params.Package...)
	if err != nil {
		return fmt.Errorf("failed to locate packages: %w", err)
	}

	var mainPackages []string
	for _, pkg := range packages {
		if pkg.Name == "main" {
			mainPackages = append(mainPackages, pkg.ImportPath)
		}
	}

	platforms := params.Platforms()

	parallelism := 1
	if params.Parallelism > 0 {
		parallelism = params.Parallelism
	}
	numBuildsTotal := len(mainPackages) * len(platforms)
	if parallelism > numBuildsTotal {
		parallelism = numBuildsTotal
	}
	fmt.Printf("running %d build(s) in parallel...\n\n", parallelism)
	semaphore := make(chan struct{}, parallelism)

	var wg sync.WaitGroup
	for _, pkg := range mainPackages {
		for _, platform := range platforms {
			buildID := ID{Platform: platform, Package: pkg}

			if containsPlatform(params.SkipPlatforms, platform) {
				statusCh <- Status{
					ID:     buildID,
					Status: "skipped",
					Data:   "included in skip_platforms",
				}
				continue
			}

			wg.Add(1)
			semaphore <- struct{}{}

			statusCh <- Status{
				ID:     buildID,
				Status: "start",
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
				statusCh <- Status{
					ID:     buildID,
					Status: "error",
					Data:   err.Error(),
				}
				<-semaphore
				continue
			}
			if platform.OS == "windows" {
				binaryName.WriteString(".exe")
			}
			buildOptions := Options{
				ID:       buildID,
				Output:   filepath.Join(outputDir, binaryName.String()),
				Gopath:   gopathDir,
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
				statusCh <- buildSingle(mod, buildOptions)

				<-semaphore
				wg.Done()
			}()
		}
	}
	wg.Wait()
	return nil
}

func buildSingle(mod Module, opts Options) Status {
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

	cmd.Env = []string{
		"GOPATH=" + opts.Gopath,
		"GOCACHE=" + filepath.Join(opts.Gopath, "cache"),
		"GOOS=" + opts.Platform.OS,
		"GOARCH=" + opts.Platform.Arch,
	}
	if opts.Cgo {
		cmd.Env = append(cmd.Env, "CGO_ENABLED=1")
	} else {
		cmd.Env = append(cmd.Env, "CGO_ENABLED=0")
	}

	err := mod.Execute(cmd)
	if err != nil {
		var execErr module.ExecutionError
		if errors.As(err, &execErr) && strings.Contains(execErr.Stderr, "cmd/go: unsupported GOOS/GOARCH pair") {
			return Status{
				ID:     opts.ID,
				Status: "skipped",
				Data:   "unsupported platform",
			}
		}

		return Status{
			ID:     opts.ID,
			Status: "error",
			Data:   err.Error(),
		}
	}
	return Status{
		ID:     opts.ID,
		Status: "success",
	}
}
