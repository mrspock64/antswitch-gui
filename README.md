# antswitch-gui

Liten fristående GUI-klient (Go + [Fyne](https://fyne.io)) för att styra en
AT-14 fjärrstyrd antennswitch (ESP32/TTGO T-Display firmware,
[mrspock64/antenna-switch](https://github.com/mrspock64/antenna-switch)) över
nätverket.

Mörkt kortbaserat gränssnitt med orange accentfärg, stor tydlig
"aktiv antenn"-text och fyra knappar för antennvalen (namnen hämtas från
enhetens API, inte hårdkodade). Automatisk polling håller vyn uppdaterad om
antennen byts från webb-UI, MQTT eller enhetens egna knappar.

Fönstret är resizbart och har två lägen (växlas med ⛶-knappen uppe till
höger, valet sparas):

- **Full** — kort med aktiv antenn, status och antennknappar i grid.
- **Mini** — smal horisontell rad (dot, aktiv antenn, fyra knappar) att ha
  liggande bredvid t.ex. radio-mjukvara medan du kör.

## Köra

```bash
go run .
```

Första gången öppnas ingen anslutning förrän du satt värdadress och API-token
via kugghjulsknappen uppe till höger. Inställningarna sparas i
`~/.antswitch-gui.json`.

- **Host**: enhetens IP (t.ex. `192.168.1.50`) eller mDNS-namnet
  `antennswitch.local` (kräver Bonjour/mDNS på klientmaskinen — inbyggt på
  macOS, kan kräva extra konfiguration på Windows/Linux; ange IP manuellt om
  mDNS inte fungerar).
- **Token**: samma `API_TOKEN` som är konfigurerad i firmwarens `config.h`.

## Bygga

Native build (kräver CGO/OpenGL-toolchain för värdplattformen, vilket Fyne
behöver):

```bash
go build -o antswitch-gui .
```

### Paketera som .app (macOS) — inget terminalfönster

Kör direkt via `go build` öppnas ett terminalfönster tillsammans med appen.
Paketera istället en riktig `.app`-bunt med `fyne package` (dubbelklickbar,
ingen terminal, egen ikon från `Icon.png`):

```bash
go install fyne.io/fyne/v2/cmd/fyne@latest
fyne package -os darwin -icon Icon.png -name AntSwitch -appID net.sm7iun.antswitch-gui
```

Detta skapar `AntSwitch.app`. Flytta den till `/Applications` om du vill ha
den i Launchpad/Spotlight.

### Korskompilering

Fyne-appar länkar mot OpenGL via CGO, så ren `GOOS=... go build` fungerar
inte rakt av mellan plattformar. Använd
[fyne-cross](https://github.com/fyne-io/fyne-cross) (kräver Docker) för att
bygga macOS-, Windows- och Linux-binärer från samma källkod:

```bash
go install fyne.io/fyne/v2/cmd/fyne-cross@latest

fyne-cross darwin -arch=amd64,arm64
fyne-cross windows -arch=amd64
fyne-cross linux -arch=amd64
```

Resultaten hamnar i `fyne-cross/dist/<plattform>/`.

## Ljust/mörkt läge

☀/🌙-knappen uppe till höger växlar tema. Valet sparas i configen.

## Auto-uppdatering

Appen kollar automatiskt (tyst, i bakgrunden) mot GitHub Releases
([mrspock64/antswitch-gui](https://github.com/mrspock64/antswitch-gui))
någon sekund efter start, och du kan även klicka **"Sök efter uppdatering"**
i inställningarna. Hittas en nyare version frågar appen om lov innan den
laddar ner och installerar — själva bytet sker i binären på disk (via
[minio/selfupdate](https://github.com/minio/selfupdate)), verifierat mot
SHA256-checksumman i releasens `checksums.txt`. Efter installation frågar
appen om den ska starta om direkt.

### Släppa en ny version

En pushad tagg i formen `vX.Y.Z` triggar
[.github/workflows/release.yml](.github/workflows/release.yml), som bygger
binärer för macOS (Apple Silicon/arm64), Windows och Linux, räknar ut
checksums.txt och skapar en GitHub Release med alla filer bifogade:

```bash
git tag v0.2.0
git push origin v0.2.0
```

## API

- `GET /api/status` → `{"active":0-3,"names":["Dipole","Vertical","Beam","EFHW"]}`
- `GET /api/select?ant=<0-3>&token=<API_TOKEN>` → samma svar vid lyckat val,
  401/400 med `{"error":"..."}` vid fel.
