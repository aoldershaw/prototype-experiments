package main

import (
	"encoding/json"
	"fmt"
	"strings"
)

type Platform struct {
	OS   string
	Arch string
}

func (p Platform) String() string {
	return p.OS + "/" + p.Arch
}

func (p *Platform) UnmarshalJSON(data []byte) error {
	var dst string
	if err := json.Unmarshal(data, &dst); err != nil {
		return err
	}

	parts := strings.Split(dst, "/")
	if len(parts) != 2 {
		return fmt.Errorf("platform should be of the form \"<os>/<arch>\" (e.g. \"linux/amd64\")")
	}

	*p = Platform{OS: parts[0], Arch: parts[1]}
	return nil
}

func (p Platform) MarshalJSON() ([]byte, error) {
	return json.Marshal(p.String())
}

func (p *Platform) UnmarshalText(data []byte) error {
	return json.Unmarshal(data, p)
}

func (p Platform) MarshalText() ([]byte, error) {
	return json.Marshal(p.String())
}
