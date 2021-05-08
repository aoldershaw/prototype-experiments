package build

import (
	"fmt"
	"sort"
	"time"
)

type Status struct {
	ID
	Status string
	Data   string
}

type State struct {
	Status    string
	Error     string
	Line      int
	StartTime time.Time
}

type UI struct {
	BuildStates map[ID]State
	NumLines    int
}

func NewUI() *UI {
	return &UI{BuildStates: map[ID]State{}}
}

func (ui *UI) Update(status Status) {
	state, ok := ui.BuildStates[status.ID]
	if !ok {
		state.Line = ui.NumLines
		ui.NumLines++
		fmt.Println(ui.buildLinePrefix(status.ID))
	}
	state.Status = status.Status

	var statusText string
	switch status.Status {
	case "start":
		state.StartTime = time.Now()
		statusText = "\x1b[36mbuilding\x1b[0m"
	case "success":
		statusText = fmt.Sprintf("\x1b[32mfinished\x1b[0m (%s)", time.Since(state.StartTime))
	case "error":
		state.Error = status.Data
		statusText = fmt.Sprintf("\x1b[31merrored\x1b[0m  (%s)", time.Since(state.StartTime))
	case "skipped":
		reason := status.Data
		statusText = fmt.Sprintf("\x1b[33mskipped\x1b[0m  (%s)", reason)
	}

	ui.BuildStates[status.ID] = state

	ui.setStatusText(status.ID, state.Line, statusText)
}

func (ui UI) setStatusText(buildID ID, line int, text string) {
	// scroll to line
	linesAway := ui.NumLines - line
	fmt.Printf("\x1b[%dA", linesAway)

	// skip over prefix text
	prefixTextLen := len(ui.buildLinePrefix(buildID))
	fmt.Printf("\x1b[%dC", prefixTextLen)

	// clear to right
	fmt.Print("\x1b[K")

	// print text
	fmt.Print(text)

	// reset cursor
	fmt.Printf("\x1b[%dB\r", linesAway)
}

func (ui UI) buildLinePrefix(buildID ID) string {
	return fmt.Sprintf("--> %15s: %s ... ", buildID.Platform, buildID.Package)
}

func (ui UI) PrintResult() {
	type BuildError struct {
		ID
		Error string
	}
	var buildErrors []BuildError
	for buildID, state := range ui.BuildStates {
		if state.Status == "error" {
			buildErrors = append(buildErrors, BuildError{
				ID:    buildID,
				Error: state.Error,
			})
		}
	}

	if len(buildErrors) == 0 {
		return
	}

	errorsWord := "errors"
	if len(buildErrors) == 1 {
		errorsWord = "error"
	}
	fmt.Printf("\n\x1b[1m%d %s occurred:\x1b[0m\n\n", len(buildErrors), errorsWord)

	// Sort by line so that errors appear in same order as builds
	sort.Slice(buildErrors, func(i, j int) bool {
		return ui.BuildStates[buildErrors[i].ID].Line < ui.BuildStates[buildErrors[i].ID].Line
	})

	for _, err := range buildErrors {
		fmt.Printf("--> %15s: %s: %s\n\n", err.Platform, err.Package, err.Error)
	}
}
