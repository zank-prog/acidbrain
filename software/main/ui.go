package main

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func runUI() {
	myApp := app.NewWithID("com.acidbrain.app")
	window := myApp.NewWindow("ACIDBRAIN")

	var samplePattern Pattern
	samplePattern.reseed(12345)
	sampleParams := Params{
		Key:        2,
		ScaleIdx:   1,
		Steps:      32,
		Density:    0.75,
		Complexity: 0.5,
		Gate:       0.5,
		Bpm:        120,
	}

	trackerText := buildTrackerText(samplePattern, sampleParams)
	label := widget.NewLabel(trackerText)
	label.TextStyle = fyne.TextStyle{Monospace: true}

	content := container.NewVBox(label)

	window.SetContent(content)
	window.Resize(fyne.NewSize(400, 600))
	window.ShowAndRun()

}

func noteName(midiNote int) string {
	names := []string{"C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"}
	octave := midiNote/12 - 1
	name := names[midiNote%12]
	return fmt.Sprintf("%s%d", name, octave)
}

func buildTrackerText(pattern Pattern, params Params) string {
	text := "STEP  NOTE\n"
	for i := 0; i < params.Steps; i++ {
		step := pattern.realize(i, params)
		if step.Active {
			marker := ""
			if step.Accent {
				marker += " *"
			}
			if step.Slide {
				marker += " ~"
			}
			text += fmt.Sprintf("%2d    %-4s%s\n", i+1, noteName(step.Note), marker)
		} else {
			text += fmt.Sprintf("%2d    ---\n", i+1)
		}
	}
	return text
}
