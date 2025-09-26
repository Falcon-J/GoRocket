# Go Rocket Go!

Go Rocket Go is a bite-sized rocket launch challenge built with [Go](https://go.dev) and [Ebitengine](https://ebitengine.org/) to explore game development while learning the language.

## 🎮 Game Overview

Race through a frantic pre-launch countdown, mash the ignition buttons to feed your fuel reserves, then ride the flames to reach the highest altitude before gravity wins. A built-in fuel meter, particle exhaust, and a jazzy soundtrack keep the action fast and punchy.

## ✨ Features

- **Charge-to-launch loop** with Ready/Set/Go countdown animations and synthesized countdown voice cues.
- **Fuel (power) meter HUD** and **combo meter** that reward fast alternating taps.
- **Dynamic exhaust particles** and launch **screen shake** to amp up motion.
- **Auto-playing background music** plus launch / countdown / power-down SFX.
- **Persistent high score saving** so your best launch survives restarts.
- **Flight results overlay** with altitude, peak speed, tap rate, and combo stats.

## ⌨️ Controls

| Action         | Key                                             |
| -------------- | ----------------------------------------------- |
| Charge engines | `Z` or `X` (rapid alternating taps recommended) |
| Reset launch   | `R`                                             |
| Quit Window    | OS close button                                 |

## 🚀 How to Play

1. Wait for the "Ready • Set • Go" banner to finish cycling.
2. Hammer `Z` and `X` during the countdown to fill the fuel bar.
3. Once fuel is stocked, the rocket blasts off automatically.
4. Keep an eye on the altitude readout and try to beat your best score.
5. Press `R` to prep the launchpad for another run.

### Pro Tips

- Alternate `Z` and `X` taps quickly rather than spamming a single key—the cadence makes it easier to max the fuel meter.
- Watch for the power meter glow; full fuel means a longer burn after takeoff.
- Let the music cues guide you: the launch SFX triggers right as liftoff begins.

## 🛠️ Getting Started

### Prerequisites

- Go **1.21+** (earlier versions may work but aren't tested)
- A sound-capable environment (the game streams MP3 audio)

### Run Locally

```cmd
cd path\to\go-rocket-go
go run .
```
