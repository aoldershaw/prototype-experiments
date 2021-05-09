package build

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/aoldershaw/prototype-experiments/go/module"
	"github.com/stretchr/testify/require"
)

type Cmd struct {
	Args []string
	Env  []string
}

type fakeModule struct {
	packages map[string][]module.Package
	cmds     []Cmd
}

func (m *fakeModule) ResolvePackages(packages ...string) ([]module.Package, error) {
	var out []module.Package
	for _, pkg := range packages {
		cur, ok := m.packages[pkg]
		if !ok {
			panic(fmt.Sprintf("missing packages definition for %q", pkg))
		}
		out = append(out, cur...)
	}
	return out, nil
}

func (m *fakeModule) Execute(cmd *exec.Cmd) error {
	m.cmds = append(m.cmds, Cmd{
		Args: cmd.Args,
		Env:  cmd.Env,
	})
	return nil
}

func TestBuild(t *testing.T) {
	const outDir = "output"
	env := func(goos, goarch, cgo string) []string {
		return []string{
			"GOPATH=" + os.Getenv("GOPATH"),
			"GOOS=" + goos,
			"GOARCH=" + goarch,
			"CGO_ENABLED=" + cgo,
		}
	}
	for _, tt := range []struct {
		desc     string
		packages map[string][]module.Package
		params   Params
		commands []Cmd
		err      string
	}{
		{
			desc: "defaults",
			packages: map[string][]module.Package{
				".": {{"main", "github.com/concourse/concourse/cmd/concourse"}},
			},
			params: Params{},
			commands: []Cmd{
				{
					Args: []string{
						"go", "build",
						// TODO: this'll break on windows (needs .exe)
						"-o", filepath.Join(outDir, "concourse-"+runtime.GOOS+"-"+runtime.GOARCH),
						"github.com/concourse/concourse/cmd/concourse",
					},
					Env: env(runtime.GOOS, runtime.GOARCH, "0"),
				},
			},
		},
		{
			desc: "multiple packages",
			packages: map[string][]module.Package{
				"./foo/...": {
					{"main", "github.com/abc/def/foo"},
					{"other", "github.com/abc/def/foo/other"},
					{"packages", "github.com/abc/def/foo/packages"},
					{"main", "github.com/abc/def/foo/other"},
				},
				"./bar/...": {
					{"main", "github.com/abc/def/bar/bar"},
				},
			},
			params: Params{
				Package: OneOrMany{"./foo/...", "./bar/..."},
			},
			commands: []Cmd{
				{
					Args: []string{
						"go", "build",
						// TODO: this'll break on windows (needs .exe)
						"-o", filepath.Join(outDir, "foo-"+runtime.GOOS+"-"+runtime.GOARCH),
						"github.com/abc/def/foo",
					},
					Env: env(runtime.GOOS, runtime.GOARCH, "0"),
				},
				{
					Args: []string{
						"go", "build",
						// TODO: this'll break on windows (needs .exe)
						"-o", filepath.Join(outDir, "other-"+runtime.GOOS+"-"+runtime.GOARCH),
						"github.com/abc/def/foo/other",
					},
					Env: env(runtime.GOOS, runtime.GOARCH, "0"),
				},
				{
					Args: []string{
						"go", "build",
						// TODO: this'll break on windows (needs .exe)
						"-o", filepath.Join(outDir, "bar-"+runtime.GOOS+"-"+runtime.GOARCH),
						"github.com/abc/def/bar/bar",
					},
					Env: env(runtime.GOOS, runtime.GOARCH, "0"),
				},
			},
		},
		{
			desc: "multiple platforms",
			packages: map[string][]module.Package{
				".": {{"main", "github.com/abc/def"}},
			},
			params: Params{
				OS:   OneOrMany{"linux", "darwin", "windows"},
				Arch: OneOrMany{"amd64", "arm64"},
				SkipPlatforms: []Platform{
					{OS: "darwin", Arch: "arm64"},
					{OS: "windows", Arch: "amd64"},
				},
			},
			commands: []Cmd{
				{
					Args: []string{
						"go", "build",
						"-o", filepath.Join(outDir, "def-linux-amd64"),
						"github.com/abc/def",
					},
					Env: env("linux", "amd64", "0"),
				},
				{
					Args: []string{
						"go", "build",
						"-o", filepath.Join(outDir, "def-linux-arm64"),
						"github.com/abc/def",
					},
					Env: env("linux", "arm64", "0"),
				},
				{
					Args: []string{
						"go", "build",
						"-o", filepath.Join(outDir, "def-darwin-amd64"),
						"github.com/abc/def",
					},
					Env: env("darwin", "amd64", "0"),
				},
				{
					Args: []string{
						"go", "build",
						"-o", filepath.Join(outDir, "def-windows-arm64.exe"),
						"github.com/abc/def",
					},
					Env: env("windows", "arm64", "0"),
				},
			},
		},
		{
			desc: "flags",
			packages: map[string][]module.Package{
				".": {{"main", "github.com/abc/def"}},
			},
			params: Params{
				OS:      OneOrMany{"linux"},
				Arch:    OneOrMany{"amd64", "arm64"},
				Ldflags: "ldflags",
				PlatformLdflags: map[Platform]string{
					{OS: "linux", Arch: "arm64"}: "ldflags-arm",
				},
				Gcflags: "gcflags",
				PlatformGcflags: map[Platform]string{
					{OS: "linux", Arch: "arm64"}: "gcflags-arm",
				},
				Asmflags: "asmflags",
				PlatformAsmflags: map[Platform]string{
					{OS: "linux", Arch: "arm64"}: "asmflags-arm",
				},
				Tags:    []string{"foo", "bar"},
				ModMode: "mod",
				Rebuild: true,
				Race:    true,
				Cgo:     true,
			},
			commands: []Cmd{
				{
					Args: []string{
						"go", "build",
						"-o", filepath.Join(outDir, "def-linux-amd64"),
						"-a",
						"-mod", "mod",
						"-race",
						"-tags", "foo,bar",
						"-ldflags", "ldflags",
						"-gcflags", "gcflags",
						"-asmflags", "asmflags",
						"github.com/abc/def",
					},
					Env: env("linux", "amd64", "1"),
				},
				{
					Args: []string{
						"go", "build",
						"-o", filepath.Join(outDir, "def-linux-arm64"),
						"-a",
						"-mod", "mod",
						"-race",
						"-tags", "foo,bar",
						"-ldflags", "ldflags-arm",
						"-gcflags", "gcflags-arm",
						"-asmflags", "asmflags-arm",
						"github.com/abc/def",
					},
					Env: env("linux", "arm64", "1"),
				},
			},
		},
	} {
		mod := &fakeModule{packages: tt.packages}
		err := build(mod, tt.params, outDir, make(chan Status, 1000))
		if tt.err != "" {
			require.EqualError(t, err, tt.err)
		} else {
			require.NoError(t, err)
		}
		require.ElementsMatch(t, tt.commands, mod.cmds)
	}
}
