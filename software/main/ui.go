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

	trackerText := buildTrackerText()
	label := widget.NewLabel(trackerText)
	label.TextStyle = fyne.TextStyle{Monospace: true}

	content := container.NewVBox(label)

	window.SetContent(content)
	window.Resize(fyne.NewSize(400, 600))
	window.ShowAndRun()
}

func buildTrackerText() string {
	text := "STEP	NOTE\n"
	for i := 0; i < 32; i++ {
		text += fmt.Sprintf("%2d	---\n", i+1)
	}
	return text
}
