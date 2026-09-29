package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
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

var (
	fullWindowSize = fyne.NewSize(420, 300)
	miniWindowSize = fyne.NewSize(460, 78)
)

// antState is the mutable UI/runtime state for the running app. Exactly one
// device profile (AT-14 or AS-1289) is active/polled at a time, selected by
// cfg.Device; switching profiles resets names/buttons for the new port count.
type antState struct {
	cfg          Config
	at14Client   *AntennaSwitchClient
	as1289Client *AS1289Client

	app fyne.App
	win fyne.Window

	statusDot   *statusDot
	statusLabel *widget.Label
	activeText  *canvas.Text
	buttons     []*widget.Button

	names   []string
	active  int
	online  bool
	pending bool
}

// portCount returns how many antenna ports the current device profile has.
func (st *antState) portCount() int {
	if st.cfg.Device == DeviceAS1289 {
		return as1289PortCount
	}
	return 4
}

// currentHost returns the host/IP for whichever profile is active.
func (st *antState) currentHost() string {
	if st.cfg.Device == DeviceAS1289 {
		return st.cfg.AS1289.Host
	}
	return st.cfg.AT14.Host
}

// resetNamesPlaceholder resets names/active to generic placeholders sized
// for the current profile's port count — used at startup and whenever the
// device profile is switched, so buttons never index past st.names.
func (st *antState) resetNamesPlaceholder() {
	n := st.portCount()
	names := make([]string, n)
	for i := range names {
		names[i] = fmt.Sprintf("Ant %d", i+1)
	}
	st.names = names
	st.active = -1
}

// nameFor safely reads st.names[i], falling back to a generic label if the
// slice hasn't caught up yet (e.g. mid device-switch).
func (st *antState) nameFor(i int) string {
	if i < len(st.names) {
		return st.names[i]
	}
	return fmt.Sprintf("Ant %d", i+1)
}

// fetchStatus polls whichever device profile is currently active.
func (st *antState) fetchStatus(ctx context.Context) (*DeviceStatus, error) {
	if st.cfg.Device == DeviceAS1289 {
		return st.as1289Client.GetStatus(ctx, st.cfg.AS1289.Host, st.cfg.AS1289.AuthUser, st.cfg.AS1289.AuthPass, st.cfg.AS1289.Names)
	}
	s, err := st.at14Client.GetStatus(ctx, st.cfg.AT14.Host)
	if err != nil {
		return nil, err
	}
	return &DeviceStatus{Active: s.Active, Names: s.Names}, nil
}

// doSelectAntenna selects antenna idx (0-based) on whichever device profile
// is currently active.
func (st *antState) doSelectAntenna(ctx context.Context, idx int) (*DeviceStatus, error) {
	if st.cfg.Device == DeviceAS1289 {
		return st.as1289Client.SelectAntenna(ctx, st.cfg.AS1289.Host, st.cfg.AS1289.AuthUser, st.cfg.AS1289.AuthPass, idx, st.cfg.AS1289.Names)
	}
	s, err := st.at14Client.SelectAntenna(ctx, st.cfg.AT14.Host, st.cfg.AT14.Token, idx)
	if err != nil {
		return nil, err
	}
	return &DeviceStatus{Active: s.Active, Names: s.Names}, nil
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
		cfg:          cfg,
		at14Client:   newAntennaSwitchClient(),
		as1289Client: newAS1289Client(),
		app:          a,
		win:          w,
	}
	st.resetNamesPlaceholder()

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
	if st.currentHost() != "" {
		go st.refreshStatus()
	}
	if st.cfg.Device == DeviceAS1289 {
		go st.refreshAS1289Names()
	}
}

// applyAS1289NamesToUI pushes freshly fetched names onto the button row /
// active-antenna display, without touching status/connection state.
func (st *antState) applyAS1289NamesToUI(names []string) {
	st.names = displayNames(names, as1289PortCount)
	for i, btn := range st.buttons {
		if i < len(st.names) {
			btn.SetText(st.names[i])
		}
	}
	if st.active >= 0 && st.active < len(st.names) {
		st.activeText.Text = st.names[st.active]
		st.activeText.Refresh()
	}
}

// refreshAS1289Names scrapes antenna names off the device's own status page
// (they aren't in the polled status protocol) using the currently saved
// AS-1289 config, and persists+applies whatever it finds. Meant to run once
// per profile setup/switch, not on every poll — names change rarely. Silent
// on failure: the periodic status poll already surfaces connectivity
// errors, and the UI just keeps showing the last-known names.
func (st *antState) refreshAS1289Names() {
	host, user, pass := st.cfg.AS1289.Host, st.cfg.AS1289.AuthUser, st.cfg.AS1289.AuthPass

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()
	names, err := st.as1289Client.FetchNames(ctx, host, user, pass)
	if err != nil {
		return
	}

	fyne.Do(func() {
		if st.cfg.Device != DeviceAS1289 {
			return // switched away while the fetch was in flight
		}
		st.cfg.AS1289.Names = names
		_ = saveConfig(st.cfg)
		st.applyAS1289NamesToUI(names)
	})
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

	n := st.portCount()
	st.buttons = make([]*widget.Button, n)
	grid := container.NewGridWithColumns(2)
	for i := 0; i < n; i++ {
		idx := i
		btn := widget.NewButton(st.nameFor(i), func() { st.selectAntenna(idx) })
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

	n := st.portCount()
	st.buttons = make([]*widget.Button, n)
	grid := container.NewGridWithColumns(n)
	for i := 0; i < n; i++ {
		idx := i
		btn := widget.NewButton(st.nameFor(i), func() { st.selectAntenna(idx) })
		st.buttons[i] = btn
		grid.Add(btn)
	}

	controls := container.NewHBox(st.themeToggleButton(), st.modeToggleButton(), st.settingsButton())

	row := container.NewBorder(nil, nil, left, controls, grid)
	return container.NewPadded(row)
}

const (
	deviceOptionAT14   = "AT-14"
	deviceOptionAS1289 = "AS-1289 (5-port)"
)

// showSettings opens Settings in its own OS window rather than an in-canvas
// dialog. In mini mode the main window is very short (460x78) — a dialog
// docked to it would force the whole window to balloon out to fit the
// dialog's content and not shrink back down afterwards, which looks broken.
// A separate window sidesteps that entirely.
//
// Only one device profile is ever active/polled at a time; the picker here
// just swaps which profile's fields are shown and, on Save, which one the
// rest of the app talks to.
func (st *antState) showSettings() {
	settingsWin := st.app.NewWindow("AntSwitch by SA0LEK — Settings")

	at14HostEntry := widget.NewEntry()
	at14HostEntry.SetText(st.cfg.AT14.Host)
	at14HostEntry.SetPlaceHolder("antennswitch.local or 192.168.1.50")

	at14TokenEntry := widget.NewPasswordEntry()
	at14TokenEntry.SetText(st.cfg.AT14.Token)
	at14TokenEntry.SetPlaceHolder("API token")

	at14Panel := widget.NewForm(
		widget.NewFormItem("Host", at14HostEntry),
		widget.NewFormItem("Token", at14TokenEntry),
	)

	as1289HostEntry := widget.NewEntry()
	as1289HostEntry.SetText(st.cfg.AS1289.Host)
	as1289HostEntry.SetPlaceHolder(as1289DefaultHost)

	as1289UserEntry := widget.NewEntry()
	as1289UserEntry.SetText(st.cfg.AS1289.AuthUser)
	as1289UserEntry.SetPlaceHolder("optional")

	as1289PassEntry := widget.NewPasswordEntry()
	as1289PassEntry.SetText(st.cfg.AS1289.AuthPass)
	as1289PassEntry.SetPlaceHolder("optional")

	// Antenna names are read from the device itself (/setswitch.htm), not
	// typed in here — this label just shows the last-known values, and the
	// button below re-fetches on demand. Save also always re-fetches using
	// whatever Host/Username/Password ends up saved, so a fresh setup or a
	// name changed on the device's own /settings page is picked up without
	// extra steps.
	as1289NamesLabel := widget.NewLabel(strings.Join(displayNames(st.cfg.AS1289.Names, as1289PortCount), ", "))
	as1289NamesLabel.Wrapping = fyne.TextWrapWord

	var as1289RefreshBtn *widget.Button
	as1289RefreshBtn = widget.NewButton("Refresh names from device", func() {
		as1289RefreshBtn.Disable()
		host, user, pass := as1289HostEntry.Text, as1289UserEntry.Text, as1289PassEntry.Text
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
			defer cancel()
			names, err := st.as1289Client.FetchNames(ctx, host, user, pass)
			fyne.Do(func() {
				as1289RefreshBtn.Enable()
				if err != nil {
					dialog.ShowError(err, settingsWin)
					return
				}
				as1289NamesLabel.SetText(strings.Join(displayNames(names, as1289PortCount), ", "))
			})
		}()
	})

	as1289Panel := widget.NewForm(
		widget.NewFormItem("Host", as1289HostEntry),
		widget.NewFormItem("Username", as1289UserEntry),
		widget.NewFormItem("Password", as1289PassEntry),
		widget.NewFormItem("Names", container.NewVBox(as1289NamesLabel, as1289RefreshBtn)),
	)

	deviceSelect := widget.NewSelect([]string{deviceOptionAT14, deviceOptionAS1289}, nil)
	deviceSelect.OnChanged = func(v string) {
		if v == deviceOptionAS1289 {
			at14Panel.Hide()
			as1289Panel.Show()
		} else {
			as1289Panel.Hide()
			at14Panel.Show()
		}
	}
	if st.cfg.Device == DeviceAS1289 {
		deviceSelect.SetSelected(deviceOptionAS1289)
	} else {
		deviceSelect.SetSelected(deviceOptionAT14)
	}

	versionLabel := widget.NewLabel("AntSwitch by SA0LEK · " + Version)
	checkUpdateBtn := widget.NewButton("Check for updates", func() {
		go st.checkForUpdates(true)
	})
	updateRow := container.NewBorder(nil, nil, versionLabel, checkUpdateBtn)

	cancelBtn := widget.NewButton("Cancel", func() { settingsWin.Close() })
	saveBtn := widget.NewButton("Save", func() {
		newDevice := DeviceAT14
		if deviceSelect.Selected == deviceOptionAS1289 {
			newDevice = DeviceAS1289
		}
		deviceChanged := newDevice != st.cfg.Device

		st.cfg.Device = newDevice
		st.cfg.AT14.Host = at14HostEntry.Text
		st.cfg.AT14.Token = at14TokenEntry.Text
		st.cfg.AS1289.Host = as1289HostEntry.Text
		st.cfg.AS1289.AuthUser = as1289UserEntry.Text
		st.cfg.AS1289.AuthPass = as1289PassEntry.Text

		if err := saveConfig(st.cfg); err != nil {
			dialog.ShowError(err, settingsWin)
			return
		}
		if deviceChanged {
			st.resetNamesPlaceholder()
			st.rebuildUI()
		} else {
			go st.refreshStatus()
			if newDevice == DeviceAS1289 {
				go st.refreshAS1289Names()
			}
		}
		settingsWin.Close()
	})
	saveBtn.Importance = widget.HighImportance
	buttonRow := container.NewHBox(layout.NewSpacer(), cancelBtn, saveBtn)

	scrollArea := container.NewVScroll(container.NewVBox(
		widget.NewLabel("Device"), deviceSelect,
		widget.NewSeparator(),
		at14Panel, as1289Panel,
	))
	scrollArea.SetMinSize(fyne.NewSize(360, 220))

	// updateRow/buttonRow stay outside the scroll area so "Check for
	// updates" and Save/Cancel are always visible, regardless of how tall
	// the device-specific panel above gets (the AS-1289 one has more rows
	// than AT-14's) or how far the window gets resized.
	bottomArea := container.NewVBox(widget.NewSeparator(), updateRow, buttonRow)
	content := container.NewBorder(nil, bottomArea, nil, nil, scrollArea)
	settingsWin.SetContent(container.NewPadded(content))
	settingsWin.Resize(fyne.NewSize(380, 460))
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
	host := st.currentHost()
	if host == "" {
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
	status, err := st.fetchStatus(ctx)

	fyne.Do(func() {
		if err != nil {
			st.online = false
			st.statusDot.setColor(colorError)
			st.statusLabel.SetText(fmt.Sprintf("Offline — %v", err))
			return
		}
		st.online = true
		st.statusDot.setColor(colorSuccess)
		st.statusLabel.SetText(fmt.Sprintf("Connected — %s", host))
		st.applyStatus(status)
	})
}

func (st *antState) applyStatus(status *DeviceStatus) {
	if len(status.Names) == len(st.buttons) {
		st.names = status.Names
	}
	st.active = status.Active

	for i, btn := range st.buttons {
		btn.SetText(st.nameFor(i))
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
	if st.pending || st.currentHost() == "" {
		return
	}
	st.pending = true
	st.setButtonsEnabled(false)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		defer cancel()
		status, err := st.doSelectAntenna(ctx, idx)

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
			st.statusLabel.SetText(fmt.Sprintf("Connected — %s", st.currentHost()))
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
