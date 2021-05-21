package build

import (
	"crypto/sha256"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
)

func computeSHA256Sum(file string) error {
	srcFile, err := os.Open(file)
	if err != nil {
		return fmt.Errorf("failed to open output file for computing sha256sum: %w", err)
	}
	defer srcFile.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, srcFile); err != nil {
		return fmt.Errorf("failed to compute sha256sum: %w", err)
	}
	sum := fmt.Sprintf("%x %s", hasher.Sum(nil), filepath.Base(file))

	outPath := file + ".sha256sum"
	if err := ioutil.WriteFile(outPath, []byte(sum), 0755); err != nil {
		return fmt.Errorf("failed to write sha256sum to file: %w", err)
	}

	return nil
}
