# AntSwitch by SA0LEK

Small standalone GUI client (Go + [Fyne](https://fyne.io)) for controlling a
remote antenna switch over the network. Two device profiles are supported —
only one is ever active/polled at a time, switched in Settings:

- **AT-14** — ESP32/TTGO T-Display firmware,
  [mrspock64/antenna-switch](https://github.com/mrspock64/antenna-switch), a
  4-port switch with a JSON HTTP API. Antenna names are fetched live from
  the device.
- **AS-1289** — a Microbit AS-1289 5-port HF switch with built-in network
  control (no separate firmware project). It has no CORS header, so it can
  only be driven from a native HTTP client like this app, not a browser
  page. Its protocol doesn't expose antenna names, so those are entered
  manually in Settings; it optionally supports HTTP Basic Auth if the
  device's own web password protection is turned on.

Dark, card-based interface with an orange accent color, a large "active
antenna" display, and one button per antenna port. Automatic polling keeps
the view in sync if the antenna is switched from elsewhere (the device's web
UI, MQTT, its own buttons, etc).

The window is resizable and has two modes (toggled with the ⛶ button top
right, the choice is saved):

- **Full** — cards with active antenna, status, and antenna buttons in a grid.
- **Mini** — a slim horizontal bar (dot, active antenna, four buttons) to
  keep alongside e.g. your radio software while operating.

## Running

```bash
go run .
```

The first time, no connection is made until you configure a device via the
gear button top right. Settings are saved to `~/.antswitch-gui.json`. Pick
**AT-14** or **AS-1289** from the Device dropdown — its fields appear below;
Save switches the app over to that profile.

**AT-14**
- **Host**: the device's IP (e.g. `192.168.1.50`) or the mDNS name
  `antennswitch.local` (requires Bonjour/mDNS on the client machine — built
  in on macOS, may need extra setup on Windows/Linux; enter the IP manually
  if mDNS doesn't work).
- **Token**: the same `API_TOKEN` configured in the firmware's `config.h`.

**AS-1289**
- **Host**: the device's IP (e.g. `192.168.86.40`).
- **Username / Password**: only needed if the device's own optional web
  password protection is enabled; leave blank otherwise.
- **Port 1–5**: the antenna name shown on each button. The device doesn't
  report these over its status protocol, so type in whatever you've named
  them on the device's own `/settings` page; a blank port shows as "Ant N".

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

### AT-14

- `GET /api/status` → `{"active":0-3,"names":["Dipole","Vertical","Beam","EFHW"]}`
- `GET /api/select?ant=<0-3>&token=<API_TOKEN>` → same response on success,
  401/400 with `{"error":"..."}` on failure.

### AS-1289

Undocumented by the manufacturer; reverse-engineered from the device's own
`/relays.js` and verified live against real hardware.

- `GET /setswitch.htm?ap<N>&ON%20&set=<unix-ms>` (N = 1–5) — selects antenna
  N; the switch is exclusive, so every other port releases automatically.
  Empty response body on success.
- `GET /setswitch.htm?upd=<unix-ms>` — the same status poll the device's own
  page uses. Response is a pipe-separated string, e.g.
  `ap1|aa2|aa3|aa4|aa5|h0|ga|l64039`: exactly one `ap<N>` token marks the
  active port (the rest are `aa<N>`, off); `h<kHz>` is the current frequency
  if a radio connection reports one; `ga`/`gp` is an automatic-band-select
  flag tied to an RRC-1258 link (unrelated to this app); `l<n>` is an
  unspecified counter.
