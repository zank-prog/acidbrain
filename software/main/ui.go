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
	trackerLabel := widget.NewLabel("")
	trackerLabel.TextStyle = fyne.TextStyle{Monospace: true}

	potLabel := widget.NewLabel("")
	potLabel.TextStyle = fyne.TextStyle{Monospace: true}

	// Buttons
	playPauseBtn := widget.NewButton("PLAY/PAUSE", func() { seq.TogglePlay() })
	stopBtn := widget.NewButton("CEASE", func() { seq.Stop() })
	regenBtn := widget.NewButton("NEW", func() { seq.Regenerate() })
	droneBtn := widget.NewButton("DRONE", func() { seq.ToggleDrone() })
	buttons := container.NewHBox(regenBtn, playPauseBtn, stopBtn, droneBtn)

	bottomArea := container.NewVBox(potLabel, buttons)

	content := container.NewBorder(
		nil,          // top
		bottomArea,   // bottom  ← pot values pinned here
		nil,          // left
		nil,          // right
		trackerLabel, // center  ← tracker fills the rest
	)

	window.SetContent(content)
	window.Resize(fyne.NewSize(480, 640))

	// Live update loop, 50ms
	go func() {
		for {
			pattern, params, step := seq.Snapshot()
			trackerText := buildTrackerText(pattern, params, step)
			potText := buildPotText(params)
			fyne.Do(func() {
				trackerLabel.SetText(trackerText)
				potLabel.SetText(potText)
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

func buildPotText(params Params) string {
	keyNames := []string{"C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"}
	return fmt.Sprintf(
		"--------------------\n"+
			"KEY    %s\n"+
			"SCALE  %s\n"+
			"STEPS  %d\n"+
			"DENS   %.2f\n"+
			"CPLX   %.2f\n"+
			"SWING  %.2f\n"+
			"ACC    %.2f\n"+
			"SLIDE  %.2f\n"+
			"GATE   %.2f\n"+
			"OCT    %d\n"+
			"BPM    %.0f",
		keyNames[params.Key%12],
		scales()[params.ScaleIdx].Name,
		params.Steps,
		params.Density,
		params.Complexity,
		params.Swing,
		params.AccentProb,
		params.SlideProb,
		params.Gate,
		params.OctaveSpan,
		params.Bpm,
	)
}
