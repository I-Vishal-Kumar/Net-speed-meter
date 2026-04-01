# Speedo — Windows Network Speed Meter

A clean, minimal, always-on-top network speed overlay for Windows.
Built with Go (backend) + Wails + Svelte (frontend), Material Design aesthetic.

---

## What We're Building

A floating desktop widget that:

* Shows real-time download / upload speed
* Updates every second with accurate timing
* Looks like a polished Material Design card
* Runs in the system tray, always-on-top optional
* Auto-detects the active network interface

---

## Tech Stack

| Layer | Choice | Why |
|---|---|---|
| Language | Go | Fast, small binary, great for system access |
| Network stats | `gopsutil/net` | Cross-platform, reliable, well-maintained |
| UI framework | **Wails v2** | Native window + web frontend, no Electron bloat |
| Frontend | **Svelte + TypeScript** | Lightweight, reactive, easy to style |
| CSS | **SMUI or hand-rolled MD3** | Material Design 3 tokens, clean look |
| Systray | `github.com/fyne-io/systray` | Maintained fork of the classic systray lib |

---

## Project Structure

```
speedo/
│
├── main.go                  # Wails app entry point
├── app.go                   # Go backend — speed logic exposed to frontend
├── monitor/
│   └── net.go               # Network interface polling
├── frontend/
│   ├── src/
│   │   ├── App.svelte        # Root component
│   │   ├── SpeedCard.svelte  # The main widget UI
│   │   ├── lib/
│   │   │   └── format.ts     # Speed formatting (B/s → KB/s → MB/s)
│   │   └── stores/
│   │       └── speed.ts      # Svelte store for live speed data
│   └── wailsjs/             # Auto-generated Go→JS bindings
├── tray/
│   └── tray.go              # System tray icon + menu
└── build/
    └── windows/
        └── icon.ico
```

---

## Core Architecture

```
┌─────────────────────────────────────────┐
│  Go Backend                             │
│                                         │
│  monitor/net.go                         │
│    └── polls gopsutil every ~1s         │
│    └── computes speed from real elapsed │
│         time (not assumed 1s)           │
│                                         │
│  app.go                                 │
│    └── exposes GetSpeed() to frontend   │
│    └── emits EventsEmit("speed", data) │
└──────────────┬──────────────────────────┘
               │ Wails event bridge
┌──────────────▼──────────────────────────┐
│  Svelte Frontend                        │
│                                         │
│  stores/speed.ts                        │
│    └── listens to "speed" event         │
│    └── updates reactive store          │
│                                         │
│  SpeedCard.svelte                       │
│    └── renders download / upload        │
│    └── animated number transitions      │
└─────────────────────────────────────────┘
```

---

## Step-by-Step Implementation Plan

### Step 1: Scaffold the Wails project

```bash
wails init -n speedo -t svelte-ts
cd speedo
```

This generates the Go + Svelte project skeleton with the Wails bridge pre-wired.

---

### Step 2: Network monitoring (Go)

**`monitor/net.go`**

Key points:
- Filter interfaces: must be Up, non-loopback, have an assigned IP
- If multiple qualify, pick the one with highest cumulative traffic
- Measure real elapsed time between samples using `time.Since()`, not assumed 1s

```go
type Sample struct {
    BytesRecv uint64
    BytesSent uint64
    At        time.Time
}

type Speed struct {
    Download float64 // bytes per second
    Upload   float64 // bytes per second
    Iface    string
}

func Poll(prev Sample) (Speed, Sample) {
    iface, curr := activeInterface()
    elapsed := time.Since(prev.At).Seconds()

    dl := float64(curr.BytesRecv-prev.BytesRecv) / elapsed
    ul := float64(curr.BytesSent-prev.BytesSent) / elapsed

    return Speed{Download: dl, Upload: ul, Iface: iface}, curr
}
```

Interface selection logic:
1. Get all interfaces via `gopsutil/net.Interfaces()`
2. Filter: flags must include "up", exclude "loopback"
3. Must have at least one non-link-local unicast address
4. Among remaining, pick highest `BytesRecv + BytesSent` (most active)

---

### Step 3: Expose to frontend via Wails

**`app.go`**

```go
func (a *App) startup(ctx context.Context) {
    a.ctx = ctx
    go a.speedLoop()
}

func (a *App) speedLoop() {
    prev := monitor.Snapshot()
    for {
        time.Sleep(1 * time.Second)
        speed, next := monitor.Poll(prev)
        prev = next
        runtime.EventsEmit(a.ctx, "speed", speed)
    }
}
```

The frontend receives push events — no polling from JS side needed.

---

### Step 4: Svelte speed store

**`frontend/src/stores/speed.ts`**

```ts
import { writable } from 'svelte/store'
import { EventsOn } from '../wailsjs/runtime'

export type SpeedData = {
  Download: number
  Upload: number
  Iface: string
}

export const speed = writable<SpeedData>({ Download: 0, Upload: 0, Iface: '' })

EventsOn('speed', (data: SpeedData) => {
  speed.set(data)
})
```

---

### Step 5: Speed formatting

**`frontend/src/lib/format.ts`**

```ts
export function formatSpeed(bps: number): { value: string; unit: string } {
  if (bps >= 1_000_000) return { value: (bps / 1_000_000).toFixed(1), unit: 'MB/s' }
  if (bps >= 1_000)     return { value: (bps / 1_000).toFixed(0),     unit: 'KB/s' }
  return                       { value: bps.toFixed(0),                unit: 'B/s'  }
}
```

---

### Step 6: The UI widget (Material Design 3)

**`frontend/src/SpeedCard.svelte`**

Design goal: small floating card, ~260×90px, Material You aesthetic.

```
┌──────────────────────────────┐
│  ↓ 12.4  MB/s   Ethernet    │
│  ↑  0.8  MB/s               │
└──────────────────────────────┘
```

Key CSS/design choices:
- Background: `surface-container` token (`#1e1e2e` dark / `#f3edf7` light)
- Accent: `primary` token (`#d0bcff` dark / `#6750a4` light)
- Font: `Roboto` or `Inter`, `font-variant-numeric: tabular-nums` (no jitter)
- Border radius: `16px` (MD3 card shape)
- Elevation: `box-shadow` matching MD3 level 2
- Download arrow: `↓` in primary color
- Upload arrow: `↑` in secondary color
- Number transitions: CSS `transition: all 0.3s ease` on the value span

Window setup in Wails (`wails.json`):
```json
{
  "width": 260,
  "height": 90,
  "frameless": true,
  "alwaysOnTop": true,
  "resizable": false,
  "backgroundColor": "#00000000"
}
```

Transparent, frameless, always-on-top. User can drag it by adding a `-webkit-app-region: drag` div as the handle.

---

### Step 7: System tray

**`tray/tray.go`** using `github.com/fyne-io/systray`

Menu items:
- Show / Hide window
- Always on top: toggle
- ── separator ──
- Quit

The tray icon can show a tiny speed indicator if you embed a dynamic icon (optional advanced feature).

---

### Step 8: Build

```bash
wails build
```

Output: `build/bin/speedo.exe` — single executable, no installer needed for basic use.

For distribution with installer: use NSIS or Inno Setup, or Wails' built-in `--nsis` flag.

---

## Key Challenges & Fixes

| Challenge | Fix |
|---|---|
| Timing drift from `time.Sleep(1s)` | Use `time.Since(prev.At)` for real elapsed time |
| Multiple network adapters | Filter by state + address, pick most active |
| VPN / virtual adapters polluting results | Exclude adapters with no real IP or zero traffic |
| UI number jitter | `tabular-nums` + CSS transitions on value |
| Window dragging (frameless) | `-webkit-app-region: drag` on title bar div |

---

## Development Order

1. **CLI first** — get accurate speed readings printing to terminal
2. **Wails scaffold** — wire up the event bridge, verify data flows to frontend
3. **Basic Svelte UI** — unstyled labels, just confirm reactivity works
4. **Style pass** — apply MD3 tokens, transitions, sizing
5. **Window chrome** — frameless, transparent, drag handle, always-on-top
6. **System tray** — show/hide, quit
7. **Polish** — icon, interface name display, dark/light theme, alerts (optional)

---

## Optional Advanced Features

- **Mini sparkline graph** — show last 30s of speed as a tiny line chart (use `Chart.js` or hand-draw on `<canvas>`)
- **Interface picker** — let user pin a specific adapter instead of auto-detect
- **Speed alerts** — notify when download drops below a set threshold
- **Theme toggle** — dark / light, respects system preference via `prefers-color-scheme`
- **Auto-launch on startup** — write a registry key or use Windows Task Scheduler
