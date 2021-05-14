package docker

import (
	"os"
	"os/exec"
)

func Start() error {
	return run("start-docker")
}

func Stop() error {
	return run("stop-docker")
}

func Load(path string) error {
	return run("docker", "load", path)
}

func run(command string, args ...string) error {
	cmd := exec.Command(command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
