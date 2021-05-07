package main

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

// locateMainPackages returns the import paths to the packages that are "main"
// packages, from the list of packages given. The list of packages can include
// relative paths, the special "..." Go keyword, etc.
func (m Module) locateMainPackages(packages ...string) ([]string, error) {
	args := make([]string, 0, len(packages)+3)
	args = append(args, "list", "-f", "{{.Name}}|{{.ImportPath}}")
	args = append(args, packages...)

	cmd := exec.Command("go", args...)
	cmd.Dir = m.Path

	output, err := execute(cmd)
	if err != nil {
		return nil, err
	}

	results := make([]string, 0, len(output))
	for _, line := range strings.Split(output, "\n") {
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "|", 2)
		if len(parts) != 2 {
			log.Printf("Bad line reading packages: %s", line)
			continue
		}

		if parts[0] == "main" {
			results = append(results, parts[1])
		}
	}

	return results, nil
}

func execute(cmd *exec.Cmd) (string, error) {
	var stderr, stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		err = fmt.Errorf("%w\nStderr: %s", err, stderr.String())
		return "", err
	}

	return stdout.String(), nil
}
