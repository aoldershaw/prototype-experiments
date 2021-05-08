package build

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/aoldershaw/prototype-experiments/go/module"
	"github.com/stretchr/testify/require"
)

type Cmd struct {
	Positional []string
	Flags      map[string]string
	Env        map[string]string
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
			return nil, fmt.Errorf("missing packages definition for %q", pkg)
		}
		out = append(out, cur...)
	}
	return out, nil
}

func (m *fakeModule) Execute(cmd *exec.Cmd) error {
	if len(cmd.Args) < 2 || cmd.Args[0] != "go" || cmd.Args[1] != "build" {
		panic("unexpected cmd " + cmd.String())
	}
	c := Cmd{
		Flags: map[string]string{},
		Env:   map[string]string{},
	}
	args := cmd.Args[2:]
	for i := 0; i < len(args); {
		if strings.HasPrefix(args[i], "-") {
			switch args[i] {
			// Zero-argument flags
			case "-a", "-race":
				c.Flags[args[i]] = ""
				i++
			default:
				c.Flags[args[i]] = args[i+1]
				i += 2
			}
		} else {
			c.Positional = append(c.Positional, args[i])
			i++
		}
	}

	for _, env := range cmd.Env {
		parts := strings.SplitN(env, "=", 2)
		c.Env[parts[0]] = parts[1]
	}

	m.cmds = append(m.cmds, c)
	return nil
}

func TestBuild(t *testing.T) {
	const outDir = "output"
	env := func(goos, goarch, cgo string) map[string]string {
		return map[string]string{
			"GOPATH":      os.Getenv("GOPATH"),
			"GOROOT":      os.Getenv("GOROOT"),
			"GOOS":        goos,
			"GOARCH":      goarch,
			"CGO_ENABLED": cgo,
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
					Positional: []string{"github.com/concourse/concourse/cmd/concourse"},
					Flags: map[string]string{
						// TODO: this'll break on windows (needs .exe)
						"-o": filepath.Join(outDir, "concourse-"+runtime.GOOS+"-"+runtime.GOARCH),
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
					Positional: []string{"github.com/abc/def/foo"},
					Flags: map[string]string{
						// TODO: this'll break on windows (needs .exe)
						"-o": filepath.Join(outDir, "foo-"+runtime.GOOS+"-"+runtime.GOARCH),
					},
					Env: env(runtime.GOOS, runtime.GOARCH, "0"),
				},
				{
					Positional: []string{"github.com/abc/def/foo/other"},
					Flags: map[string]string{
						// TODO: this'll break on windows (needs .exe)
						"-o": filepath.Join(outDir, "other-"+runtime.GOOS+"-"+runtime.GOARCH),
					},
					Env: env(runtime.GOOS, runtime.GOARCH, "0"),
				},
				{
					Positional: []string{"github.com/abc/def/bar/bar"},
					Flags: map[string]string{
						// TODO: this'll break on windows (needs .exe)
						"-o": filepath.Join(outDir, "bar-"+runtime.GOOS+"-"+runtime.GOARCH),
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
					Positional: []string{"github.com/abc/def"},
					Flags: map[string]string{
						"-o": filepath.Join(outDir, "def-linux-amd64"),
					},
					Env: env("linux", "amd64", "0"),
				},
				{
					Positional: []string{"github.com/abc/def"},
					Flags: map[string]string{
						"-o": filepath.Join(outDir, "def-linux-arm64"),
					},
					Env: env("linux", "arm64", "0"),
				},
				{
					Positional: []string{"github.com/abc/def"},
					Flags: map[string]string{
						"-o": filepath.Join(outDir, "def-darwin-amd64"),
					},
					Env: env("darwin", "amd64", "0"),
				},
				{
					Positional: []string{"github.com/abc/def"},
					Flags: map[string]string{
						"-o": filepath.Join(outDir, "def-windows-arm64.exe"),
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
					Positional: []string{"github.com/abc/def"},
					Flags: map[string]string{
						"-o":        filepath.Join(outDir, "def-linux-amd64"),
						"-ldflags":  "ldflags",
						"-gcflags":  "gcflags",
						"-asmflags": "asmflags",
						"-tags":     "foo,bar",
						"-mod":      "mod",
						"-a":        "",
						"-race":     "",
					},
					Env: env("linux", "amd64", "1"),
				},
				{
					Positional: []string{"github.com/abc/def"},
					Flags: map[string]string{
						"-o":        filepath.Join(outDir, "def-linux-arm64"),
						"-ldflags":  "ldflags-arm",
						"-gcflags":  "gcflags-arm",
						"-asmflags": "asmflags-arm",
						"-tags":     "foo,bar",
						"-mod":      "mod",
						"-a":        "",
						"-race":     "",
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
