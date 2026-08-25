package main

import (
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
)

// newCard wraps content in a rounded, slightly raised panel.
func newCard(content fyne.CanvasObject) *fyne.Container {
	_, card, _, _, _, border := currentPalette()
	bg := canvas.NewRectangle(card)
	bg.CornerRadius = 12
	bg.StrokeColor = border
	bg.StrokeWidth = 1
	return container.NewStack(bg, container.NewPadded(content))
}

// newCaption is a small, muted, uppercase label used as a section header.
func newCaption(text string) *canvas.Text {
	_, _, _, _, muted, _ := currentPalette()
	t := canvas.NewText(strings.ToUpper(text), muted)
	t.TextSize = 11
	t.TextStyle = fyne.TextStyle{Bold: true}
	return t
}

// newBigText is a large, bold, colored value display (e.g. the active antenna name).
func newBigText(text string, col color.Color) *canvas.Text {
	t := canvas.NewText(text, col)
	t.TextSize = 26
	t.TextStyle = fyne.TextStyle{Bold: true}
	return t
}

// statusDot is a small filled circle used to show online/offline state.
type statusDot struct {
	circle *canvas.Circle
}

func newStatusDot() *statusDot {
	_, _, _, _, muted, _ := currentPalette()
	c := canvas.NewCircle(muted)
	c.StrokeWidth = 0
	return &statusDot{circle: c}
}

func (d *statusDot) object() fyne.CanvasObject {
	wrapped := container.NewGridWrap(fyne.NewSize(10, 10), d.circle)
	return container.New(&centerVLayout{}, wrapped)
}

func (d *statusDot) setColor(col color.Color) {
	d.circle.FillColor = col
	d.circle.Refresh()
}

// centerVLayout vertically centers a single child within whatever height it's given.
type centerVLayout struct{}

func (l *centerVLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if len(objects) == 0 {
		return fyne.NewSize(0, 0)
	}
	return objects[0].MinSize()
}

func (l *centerVLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) == 0 {
		return
	}
	o := objects[0]
	min := o.MinSize()
	o.Resize(min)
	o.Move(fyne.NewPos(0, (size.Height-min.Height)/2))
}
