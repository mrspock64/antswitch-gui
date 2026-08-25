package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const pollInterval = 2500 * time.Millisecond

const antennaCount = 4

var (
	fullWindowSize = fyne.NewSize(420, 300)
	miniWindowSize = fyne.NewSize(460, 78)
)

// antState is the mutable UI/runtime state for the running app.
type antState struct {
	cfg    Config
	client *AntennaSwitchClient

	app fyne.App
	win fyne.Window

	statusDot   *statusDot
	statusLabel *widget.Label
	activeText  *canvas.Text
	buttons     [antennaCount]*widget.Button

	names   []string
	active  int
	online  bool
	pending bool
}

func main() {
	app.SetMetadata(fyne.AppMetadata{
		ID:         "net.sm7iun.antswitch-gui",
		Name:       "AntSwitch",
		Migrations: map[string]bool{"fyneDo": true},
	})

	a := app.NewWithID("net.sm7iun.antswitch-gui")

	cfg := loadConfig()
	a.Settings().SetTheme(newAntSwitchTheme(cfg.Light))

	w := a.NewWindow("AntSwitch by SA0LEK")

	st := &antState{
		cfg:    cfg,
		client: newAntennaSwitchClient(),
		app:    a,
		win:    w,
		names:  []string{"Ant 1", "Ant 2", "Ant 3", "Ant 4"},
		active: -1,
	}

	st.rebuildUI()

	ctx, cancel := context.WithCancel(context.Background())
	w.SetOnClosed(cancel)

	w.Show()
	go st.pollLoop(ctx)
	go func() {
		time.Sleep(4 * time.Second)
		st.checkForUpdates(false)
	}()

	a.Run()
}

// rebuildUI (re)builds the window content for the current mode (mini/full)
// and re-creates the widgets that refreshStatus/applyStatus write to.
func (st *antState) rebuildUI() {
	if st.cfg.Mini {
		st.win.SetContent(st.buildMiniUI())
		st.win.Resize(miniWindowSize)
		st.win.SetFixedSize(false)
	} else {
		st.win.SetContent(st.buildFullUI())
		st.win.Resize(fullWindowSize)
		st.win.SetFixedSize(false)
	}
	if st.cfg.Host != "" {
		go st.refreshStatus()
	}
}

func (st *antState) modeToggleButton() *widget.Button {
	icon := theme.ViewFullScreenIcon()
	if st.cfg.Mini {
		icon = theme.ViewRestoreIcon()
	}
	return widget.NewButtonWithIcon("", icon, func() {
		st.cfg.Mini = !st.cfg.Mini
		_ = saveConfig(st.cfg)
		st.rebuildUI()
	})
}

func (st *antState) settingsButton() *widget.Button {
	return widget.NewButtonWithIcon("", theme.SettingsIcon(), st.showSettings)
}

func (st *antState) themeToggleButton() *widget.Button {
	label := "🌙"
	if st.cfg.Light {
		label = "☀"
	}
	return widget.NewButton(label, func() {
		st.cfg.Light = !st.cfg.Light
		_ = saveConfig(st.cfg)
		st.app.Settings().SetTheme(newAntSwitchTheme(st.cfg.Light))
		st.rebuildUI()
	})
}

func (st *antState) buildFullUI() fyne.CanvasObject {
	_, _, _, fg, _, _ := currentPalette()
	title := canvas.NewText("ANTSWITCH", fg)
	title.TextSize = 16
	title.TextStyle = fyne.TextStyle{Bold: true}

	headerRow := container.NewBorder(nil, nil, title,
		container.NewHBox(st.themeToggleButton(), st.modeToggleButton(), st.settingsButton()))

	st.statusDot = newStatusDot()
	st.statusLabel = widget.NewLabel("Offline")
	st.statusLabel.Wrapping = fyne.TextWrapWord
	statusRow := container.NewHBox(st.statusDot.object(), st.statusLabel)

	st.activeText = newBigText("—", colorAccent)
	activeCard := newCard(container.NewVBox(
		newCaption("Active antenna"),
		st.activeText,
		widget.NewSeparator(),
		statusRow,
	))

	grid := container.NewGridWithColumns(2)
	for i := 0; i < antennaCount; i++ {
		idx := i
		btn := widget.NewButton(st.names[i], func() { st.selectAntenna(idx) })
		st.buttons[i] = btn
		grid.Add(btn)
	}
	buttonsCard := newCard(container.NewVBox(
		newCaption("Select antenna"),
		grid,
	))

	content := container.NewVBox(
		headerRow,
		activeCard,
		buttonsCard,
		layout.NewSpacer(),
	)

	return container.NewPadded(content)
}

func (st *antState) buildMiniUI() fyne.CanvasObject {
	st.statusDot = newStatusDot()

	st.activeText = newBigText("—", colorAccent)
	st.activeText.TextSize = 18

	st.statusLabel = widget.NewLabel("Offline")
	st.statusLabel.Hidden = true // no room in mini mode; status is conveyed by the dot

	// activeText itself isn't shown in mini mode (the highlighted button
	// already conveys the active antenna) but stays live so applyStatus
	// can keep updating it uniformly across modes.
	left := container.NewHBox(st.statusDot.object())

	grid := container.NewGridWithColumns(antennaCount)
	for i := 0; i < antennaCount; i++ {
		idx := i
		btn := widget.NewButton(st.names[i], func() { st.selectAntenna(idx) })
		st.buttons[i] = btn
		grid.Add(btn)
	}

	controls := container.NewHBox(st.themeToggleButton(), st.modeToggleButton(), st.settingsButton())

	row := container.NewBorder(nil, nil, left, controls, grid)
	return container.NewPadded(row)
}

// showSettings opens Settings in its own OS window rather than an in-canvas
// dialog. In mini mode the main window is very short (460x78) — a dialog
// docked to it would force the whole window to balloon out to fit the
// dialog's content and not shrink back down afterwards, which looks broken.
// A separate window sidesteps that entirely.
func (st *antState) showSettings() {
	hostEntry := widget.NewEntry()
	hostEntry.SetText(st.cfg.Host)
	hostEntry.SetPlaceHolder("antennswitch.local or 192.168.1.50")

	tokenEntry := widget.NewPasswordEntry()
	tokenEntry.SetText(st.cfg.Token)
	tokenEntry.SetPlaceHolder("API token")

	form := widget.NewForm(
		widget.NewFormItem("Host", hostEntry),
		widget.NewFormItem("Token", tokenEntry),
	)

	settingsWin := st.app.NewWindow("AntSwitch by SA0LEK — Settings")

	versionLabel := widget.NewLabel("AntSwitch by SA0LEK · " + Version)
	checkUpdateBtn := widget.NewButton("Check for updates", func() {
		go st.checkForUpdates(true)
	})
	updateRow := container.NewBorder(nil, nil, versionLabel, checkUpdateBtn)

	cancelBtn := widget.NewButton("Cancel", func() { settingsWin.Close() })
	saveBtn := widget.NewButton("Save", func() {
		st.cfg.Host = hostEntry.Text
		st.cfg.Token = tokenEntry.Text
		if err := saveConfig(st.cfg); err != nil {
			dialog.ShowError(err, settingsWin)
			return
		}
		go st.refreshStatus()
		settingsWin.Close()
	})
	saveBtn.Importance = widget.HighImportance
	buttonRow := container.NewHBox(layout.NewSpacer(), cancelBtn, saveBtn)

	content := container.NewVBox(form, widget.NewSeparator(), updateRow, buttonRow)
	settingsWin.SetContent(container.NewPadded(content))
	settingsWin.Resize(fyne.NewSize(340, 220))
	settingsWin.SetFixedSize(true)
	settingsWin.Show()
}

// checkForUpdates queries GitHub for the latest release. If manual is true
// (user clicked "Check for updates"), it also reports "already up to date"
// and any lookup errors; a silent startup check stays quiet unless an
// update is actually found.
func (st *antState) checkForUpdates(manual bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rel, err := fetchLatestRelease(ctx)
	if err != nil {
		if manual {
			fyne.Do(func() { dialog.ShowError(err, st.win) })
		}
		return
	}

	if !isNewerVersion(Version, rel.TagName) {
		if manual {
			fyne.Do(func() {
				dialog.ShowInformation("Update", fmt.Sprintf("You're running the latest version (%s).", Version), st.win)
			})
		}
		return
	}

	fyne.Do(func() {
		dialog.ShowConfirm("Update available",
			fmt.Sprintf("%s is available (you're running %s). Download and install now?", rel.TagName, Version),
			func(ok bool) {
				if ok {
					go st.downloadAndInstall(rel)
				}
			}, st.win)
	})
}

func (st *antState) downloadAndInstall(rel *githubRelease) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	err := applyUpdate(ctx, rel)

	fyne.Do(func() {
		if err != nil {
			dialog.ShowError(err, st.win)
			return
		}
		dialog.ShowConfirm("Update installed",
			"Restart AntSwitch now to use the new version?",
			func(ok bool) {
				if ok {
					relaunchAndExit()
				}
			}, st.win)
	})
}

func relaunchAndExit() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	cmd := exec.Command(exe)
	_ = cmd.Start()
	os.Exit(0)
}

func (st *antState) pollLoop(ctx context.Context) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			st.refreshStatus()
		}
	}
}

func (st *antState) refreshStatus() {
	if st.cfg.Host == "" {
		fyne.Do(func() {
			st.online = false
			_, _, _, _, muted, _ := currentPalette()
			st.statusDot.setColor(muted)
			st.statusLabel.SetText("No host configured — open settings")
		})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()
	status, err := st.client.GetStatus(ctx, st.cfg.Host)

	fyne.Do(func() {
		if err != nil {
			st.online = false
			st.statusDot.setColor(colorError)
			st.statusLabel.SetText(fmt.Sprintf("Offline — %v", err))
			return
		}
		st.online = true
		st.statusDot.setColor(colorSuccess)
		st.statusLabel.SetText(fmt.Sprintf("Connected — %s", st.cfg.Host))
		st.applyStatus(status)
	})
}

func (st *antState) applyStatus(status *StatusResponse) {
	if len(status.Names) == antennaCount {
		st.names = status.Names
	}
	st.active = status.Active

	for i, btn := range st.buttons {
		btn.SetText(st.names[i])
		if i == st.active {
			btn.Importance = widget.HighImportance
		} else {
			btn.Importance = widget.MediumImportance
		}
		btn.Refresh()
	}

	if st.active >= 0 && st.active < len(st.names) {
		st.activeText.Text = st.names[st.active]
	} else {
		st.activeText.Text = "—"
	}
	st.activeText.Refresh()
}

func (st *antState) selectAntenna(idx int) {
	if st.pending || st.cfg.Host == "" {
		return
	}
	st.pending = true
	st.setButtonsEnabled(false)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		defer cancel()
		status, err := st.client.SelectAntenna(ctx, st.cfg.Host, st.cfg.Token, idx)

		fyne.Do(func() {
			st.pending = false
			st.setButtonsEnabled(true)
			if err != nil {
				st.online = false
				st.statusDot.setColor(colorError)
				st.statusLabel.SetText(fmt.Sprintf("Error: %v", err))
				return
			}
			st.online = true
			st.statusDot.setColor(colorSuccess)
			st.statusLabel.SetText(fmt.Sprintf("Connected — %s", st.cfg.Host))
			st.applyStatus(status)
		})
	}()
}

func (st *antState) setButtonsEnabled(enabled bool) {
	for _, btn := range st.buttons {
		if enabled {
			btn.Enable()
		} else {
			btn.Disable()
		}
	}
}
