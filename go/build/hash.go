package build

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
)

var hashers = map[string]func() hash.Hash{
	"sha1":   sha1.New,
	"sha256": sha256.New,
}

type SHASum string

func (s *SHASum) UnmarshalJSON(data []byte) error {
	{
		var enabled bool
		if err := json.Unmarshal(data, &enabled); err == nil {
			if enabled {
				*s = SHASum("sha1")
			} else {
				*s = ""
			}
			return nil
		}
	}
	{
		var algorithm string
		if err := json.Unmarshal(data, &algorithm); err == nil {
			if _, ok := hashers[algorithm]; !ok {
				return fmt.Errorf("invalid shasum algorithm: %s", algorithm)
			}
			*s = SHASum(algorithm)
			return nil
		}
	}
	return fmt.Errorf("must be either a bool or a string")
}

func computeSHASum(file string, algorithm SHASum) error {
	srcFile, err := os.Open(file)
	if err != nil {
		return fmt.Errorf("failed to open output file for computing shasum: %w", err)
	}
	defer srcFile.Close()

	hasher := hashers[string(algorithm)]()
	if _, err := io.Copy(hasher, srcFile); err != nil {
		return fmt.Errorf("failed to compute shasum: %w", err)
	}
	sum := fmt.Sprintf("%x  %s", hasher.Sum(nil), filepath.Base(file))

	outPath := file + "." + string(algorithm)
	if err := ioutil.WriteFile(outPath, []byte(sum), 0755); err != nil {
		return fmt.Errorf("failed to write shasum to file: %w", err)
	}

	return nil
}
