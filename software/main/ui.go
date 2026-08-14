package main

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func runUI(seq *Sequencer) {
	myApp := app.NewWithID("com.acidbrain.app")
	window := myApp.NewWindow("ACIDBRAIN")

	//	var samplePattern Pattern
	//	samplePattern.reseed(12345)
	//	sampleParams := Params{
	//		Key:        4,
	//		ScaleIdx:   1,
	//		Steps:      32,
	//		Density:    0.75,
	//		Complexity: 0.5,
	//		Gate:       0.5,
	//		Bpm:        120,
	//	}

	//	trackerText := buildTrackerText(samplePattern, sampleParams)
	label := widget.NewLabel("")
	label.TextStyle = fyne.TextStyle{Monospace: true}

	content := container.NewVBox(label)

	window.SetContent(content)
	window.Resize(fyne.NewSize(400, 600))

	// Live update loop, 50ms
	go func() {
		for {
			pattern, params, step := seq.Snapshot()
			text := buildTrackerText(pattern, params, step)

			fyne.Do(func() {
				label.SetText(text)
			})

			time.Sleep(50 * time.Millisecond)
		}
	}()
	window.ShowAndRun()

}

func noteName(midiNote int) string {
	names := []string{
		"C", "C#", "D", "D#", "E", "F",
		"F#", "G", "G#", "A", "A#", "B",
	}
	octave := midiNote/12 - 1
	name := names[midiNote%12]
	return fmt.Sprintf("%s%d", name, octave)
}

func buildTrackerText(pattern Pattern, params Params, currentStep int) string {
	text := "ACID	BRAIN\n"
	for i := 0; i < params.Steps; i++ {
		step := pattern.realize(i, params)

		cursor := "  "
		if i == currentStep {
			cursor = "> "
		}

		if step.Active {
			marker := ""
			if step.Accent {
				marker += " *"
			}
			if step.Slide {
				marker += " ~"
			}
			text += fmt.Sprintf("%s%2d    %-4s%s\n", cursor, i+1, noteName(step.Note), marker)
		} else {
			text += fmt.Sprintf("%s%2d    ---\n", cursor, i+1)
		}
	}
	return text
}
