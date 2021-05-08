package main

import (
	"fmt"
	"sort"
	"time"
)

type StatusUpdate struct {
	BuildID
	Status string
	Data   string
}

type BuildState struct {
	Status    string
	Error     string
	Line      int
	StartTime time.Time
	EndTime   time.Time
}

type UI struct {
	BuildStates map[BuildID]BuildState
	NumLines    int
}

func (ui *UI) Update(update StatusUpdate) {
	state, ok := ui.BuildStates[update.BuildID]
	if !ok {
		state.Line = ui.NumLines
		ui.NumLines++
		fmt.Println(ui.buildLinePrefix(update.BuildID))
	}
	state.Status = update.Status

	var statusText string
	switch update.Status {
	case "start":
		state.StartTime = time.Now()
		statusText = "\x1b[36mbuilding\x1b[0m"
	case "success":
		state.EndTime = time.Now()
		statusText = fmt.Sprintf("\x1b[32mfinished\x1b[0m (%s)", state.EndTime.Sub(state.StartTime))
	case "error":
		state.Error = update.Data
		state.EndTime = time.Now()
		statusText = fmt.Sprintf("\x1b[31merrored\x1b[0m  (%s)", state.EndTime.Sub(state.StartTime))
	}

	ui.BuildStates[update.BuildID] = state

	ui.setStatusText(update.BuildID, state.Line, statusText)
}

func (ui UI) setStatusText(buildID BuildID, line int, text string) {
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

func (ui UI) buildLinePrefix(buildID BuildID) string {
	return fmt.Sprintf("--> %15s: %s ... ", buildID.Platform, buildID.Package)
}

func (ui UI) PrintResult() {
	type BuildError struct {
		BuildID
		Error string
	}
	var buildErrors []BuildError
	for buildID, state := range ui.BuildStates {
		if state.Status == "error" {
			buildErrors = append(buildErrors, BuildError{
				BuildID: buildID,
				Error:   state.Error,
			})
		}
	}

	if len(buildErrors) == 0 {
		return
	}

	fmt.Printf("\n%d errors occurred:\n\n", len(buildErrors))

	// Sort by line so that errors appear in same order as builds
	sort.Slice(buildErrors, func(i, j int) bool {
		return ui.BuildStates[buildErrors[i].BuildID].Line < ui.BuildStates[buildErrors[i].BuildID].Line
	})

	for _, err := range buildErrors {
		fmt.Printf("--> %15s: %s errored: %s\n\n", err.Platform, err.Package, err.Error)
	}
}