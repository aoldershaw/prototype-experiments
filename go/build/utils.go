package build

import (
	"encoding/json"
	"fmt"
)

type OneOrMany []string

func (o *OneOrMany) UnmarshalJSON(data []byte) error {
	{
		var dst string
		if err := json.Unmarshal(data, &dst); err == nil {
			*o = OneOrMany{dst}
			return nil
		}
	}
	{
		var dst []string
		if err := json.Unmarshal(data, &dst); err == nil {
			*o = OneOrMany(dst)
			return nil
		}
	}
	return fmt.Errorf("must be either a string or a []string")
}
