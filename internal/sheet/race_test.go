//go:build race

package sheet

// raceDetector is set when the race detector runs, which makes the layout
// about ten times slower.
const raceDetector = true
