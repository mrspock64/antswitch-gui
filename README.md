# antswitch-gui

Small standalone GUI client (Go + [Fyne](https://fyne.io)) for controlling an
AT-14 remote antenna switch (ESP32/TTGO T-Display firmware,
[mrspock64/antenna-switch](https://github.com/mrspock64/antenna-switch)) over
the network.

Dark, card-based interface with an orange accent color, a large "active
antenna" display, and four buttons for the antenna choices (names are
fetched from the device's API, not hardcoded). Automatic polling keeps the
view in sync if the antenna is switched from the web UI, MQTT, or the
device's own buttons.

The window is resizable and has two modes (toggled with the ⛶ button top
right, the choice is saved):

- **Full** — cards with active antenna, status, and antenna buttons in a grid.
- **Mini** — a slim horizontal bar (dot, active antenna, four buttons) to
  keep alongside e.g. your radio software while operating.

## Running

```bash
go run .
```

The first time, no connection is made until you set the host address and
API token via the gear button top right. Settings are saved to
`~/.antswitch-gui.json`.

- **Host**: the device's IP (e.g. `192.168.1.50`) or the mDNS name
  `antennswitch.local` (requires Bonjour/mDNS on the client machine — built
  in on macOS, may need extra setup on Windows/Linux; enter the IP manually
  if mDNS doesn't work).
- **Token**: the same `API_TOKEN` configured in the firmware's `config.h`.

## Building

Native build (requires a CGO/OpenGL toolchain for the host platform, which
Fyne needs):

```bash
go build -o antswitch-gui .
```

### Package as a .app (macOS) — no terminal window

Running it via plain `go build` opens a terminal window alongside the app.
Instead, package a real `.app` bundle with `fyne package` (double-clickable,
no terminal, its own icon from `Icon.png`):

```bash
go install fyne.io/fyne/v2/cmd/fyne@latest
fyne package -os darwin -icon Icon.png -name AntSwitch -appID net.sm7iun.antswitch-gui
```

This creates `AntSwitch.app`. Move it to `/Applications` if you want it in
Launchpad/Spotlight.

### Cross-compiling

Fyne apps link against OpenGL via CGO, so a plain `GOOS=... go build` doesn't
work across platforms out of the box. Use
[fyne-cross](https://github.com/fyne-io/fyne-cross) (requires Docker) to
build macOS, Windows, and Linux binaries from the same source:

```bash
go install fyne.io/fyne/v2/cmd/fyne-cross@latest

fyne-cross darwin -arch=amd64,arm64
fyne-cross windows -arch=amd64
fyne-cross linux -arch=amd64
```

Output lands in `fyne-cross/dist/<platform>/`.

## Light/dark mode

The ☀/🌙 button top right toggles the theme. The choice is saved in the config.

## Windows SmartScreen / browser "virus" warning

The Windows build (`antswitch-gui-windows-amd64.exe`) isn't code-signed, so
Chrome's Safe Browsing check and Windows SmartScreen will likely flag it as
unrecognized or "dangerous" the first time anyone downloads it. This is a
**reputation-based false positive**, not a real detection — a brand-new
binary from a small project has no download history yet, and unsigned Go
binaries (statically linked, no publisher signature) trigger this heuristic
especially often. A proper fix means buying a code-signing certificate and
verifying identity with a CA; until/unless that happens, here's how to get
past the warning and confirm the file is genuinely what we built:

1. **Verify the checksum** — every release includes a `checksums.txt`.
   On Windows (PowerShell):
   ```powershell
   Get-FileHash antswitch-gui-windows-amd64.exe -Algorithm SHA256
   ```
   Compare the output to the matching line in `checksums.txt` from the same
   release. If it matches, the file is exactly what our CI build produced.
2. **In Chrome**: click the download's "..." menu → **Keep** (or **Keep
   anyway**) if prompted.
3. **In Windows SmartScreen** (when running the .exe): click **More info**,
   then **Run anyway**.

## Auto-update

The app automatically checks (silently, in the background) against GitHub
Releases ([mrspock64/antswitch-gui](https://github.com/mrspock64/antswitch-gui))
a few seconds after startup, and you can also click **"Check for updates"**
in Settings. If a newer version is found, the app asks for confirmation
before downloading and installing — the actual swap happens to the binary
on disk (via [minio/selfupdate](https://github.com/minio/selfupdate)),
verified against the SHA256 checksum in the release's `checksums.txt`.
After installation, the app asks whether to restart right away.

### Releasing a new version

Pushing a tag in the form `vX.Y.Z` triggers
[.github/workflows/release.yml](.github/workflows/release.yml), which builds
binaries for macOS (Apple Silicon/arm64), Windows, and Linux, computes
checksums.txt, and creates a GitHub Release with all the files attached:

```bash
git tag v0.2.0
git push origin v0.2.0
```

## API

- `GET /api/status` → `{"active":0-3,"names":["Dipole","Vertical","Beam","EFHW"]}`
- `GET /api/select?ant=<0-3>&token=<API_TOKEN>` → same response on success,
  401/400 with `{"error":"..."}` on failure.
