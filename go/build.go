package main

import (
	"bytes"
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

	Ldflags          string              `json:"ldflags"`
	LdflagsOverrides map[Platform]string `json:"ldflags_overrides"`

	Gcflags          string              `json:"gcflags"`
	GcflagsOverrides map[Platform]string `json:"gcflags_overrides"`

	Asmflags          string              `json:"asmflags"`
	AsmflagsOverrides map[Platform]string `json:"asmflags_overrides"`

	Tags    []string `json:"tags"`
	ModMode string   `json:"mod"`
	Rebuild bool     `json:"rebuild"`
	Race    bool     `json:"race"`
	Cgo     bool     `json:"cgo"`

	Parallelism int `json:"parallelism"`
}

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
			platform := Platform{OS: os, Arch: arch}
			if !containsPlatform(p.SkipPlatforms, platform) {
				platforms = append(platforms, platform)
			}
		}
	}
	return platforms
}

func containsPlatform(platforms []Platform, platform Platform) bool {
	for _, p := range platforms {
		if platform == p {
			return true
		}
	}
	return false
}

type OutputTemplateParams struct {
	Dir  string
	OS   string
	Arch string
}

type BuildOptions struct {
	Package  string
	Platform Platform
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

	var errsLock sync.Mutex
	var errs []string
	appendErr := func(platform Platform, err error) {
		errsLock.Lock()
		defer errsLock.Unlock()
		errs = append(errs, fmt.Sprintf("--> %15s error: %s\n", platform, err))
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

	var wg sync.WaitGroup
	for _, pkg := range mainPkgs {
		for _, platform := range platforms {
			wg.Add(1)
			go func(platform Platform, pkg string) {
				defer wg.Done()

				semaphore <- struct{}{}
				defer func() { <-semaphore }()

				fmt.Printf("--> %15s: %s\n", platform, pkg)

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
					appendErr(platform, err)
					return
				}
				if platform.OS == "windows" {
					binaryName.WriteString(".exe")
				}

				err = m.buildSingle(BuildOptions{
					Package:  pkg,
					Platform: platform,
					Output:   filepath.Join(outputDirAbs, binaryName.String()),
					Ldflags:  valueOrOverride(params.Ldflags, params.LdflagsOverrides),
					Gcflags:  valueOrOverride(params.Gcflags, params.GcflagsOverrides),
					Asmflags: valueOrOverride(params.Asmflags, params.AsmflagsOverrides),
					Tags:     params.Tags,
					ModMode:  params.ModMode,
					Rebuild:  params.Rebuild,
					Race:     params.Race,
					Cgo:      params.Cgo,
				})
				if err != nil {
					appendErr(platform, err)
					return
				}
			}(platform, pkg)
		}
	}

	wg.Wait()
	if len(errs) > 0 {
		fmt.Printf("\n%d errors occurred:\n\n", len(errs))
		for _, err := range errs {
			fmt.Println(err + "\n")
		}
	}

	return []prototype.MessageResponse{{
		Object: map[string]interface{}{
			"built": prototype.Artifact(outputDirRel),
		},
	}}, nil
}

func (m Module) buildSingle(opts BuildOptions) error {
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
	return err
}
