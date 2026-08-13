package main

import (
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

func runUI() {
	myApp := app.New()
	window := myApp.NewWindow("ACIDBRAIN")

	content := container.NewVBox(
		widget.NewLabel("ACIDBRAIN TRACKER"),
	)

	window.SetContent(content)
	window.ShowAndRun()
}
