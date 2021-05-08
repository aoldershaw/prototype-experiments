package module

import (
	"bytes"
	"fmt"
	"log"
	"os/exec"
	"strings"
)

type Module struct {
	Path string `json:"module" prototype:"required"`
}

type Package struct {
	Name       string
	ImportPath string
}

// ResolvePackages returns the import paths to the packages that are "main"
// packages, from the list of packages given. The list of packages can include
// relative paths, the special "..." Go keyword, etc.
func (m Module) ResolvePackages(packages ...string) ([]Package, error) {
	args := make([]string, 0, len(packages)+3)
	args = append(args, "list", "-f", "{{.Name}}|{{.ImportPath}}")
	args = append(args, packages...)

	cmd := exec.Command("go", args...)

	output, err := m.Execute(cmd)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(output, "\n")
	results := make([]Package, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "|", 2)
		if len(parts) != 2 {
			log.Printf("Bad line reading packages: %s", line)
			continue
		}

		if parts[0] == "main" {
			results = append(results, Package{
				Name:       parts[0],
				ImportPath: parts[1],
			})
		}
	}

	return results, nil
}

func (m Module) Execute(cmd *exec.Cmd) (string, error) {
	var stderr, stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Dir = m.Path

	if err := cmd.Run(); err != nil {
		return "", ExecutionError{
			Err:    err,
			Stderr: stderr.String(),
		}
	}

	return stdout.String(), nil
}

type ExecutionError struct {
	Err    error
	Stderr string
}

func (e ExecutionError) Error() string {
	return fmt.Sprintf("%s\n%s", e.Err, e.Stderr)
}
