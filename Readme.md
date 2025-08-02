# Winmove

`winmove` is a Go script for moving and resizing windows between monitors on Linux. It supports positioning windows in half the screen on the current monitor and automatically moves the window to the adjacent monitor when it is at the edge of the screen.

---

## Features

- Detects the current monitor of the active window.
- Moves and resizes the window to occupy half the width of the monitor (left or right).
- **When the window is positioned at the extreme left or right edge of the monitor, the script moves the window to the adjacent monitor, keeping the positioning on the indicated half.**
- Uses the X11 protocol and RandR extension to manage monitors and windows.

---

## Requirements

- Linux with X11 server.
- Go installed.
- Go libraries:
  - `github.com/BurntSushi/xgb`
  - `github.com/BurntSushi/xgb/randr`

To install the Go libraries, run:

```bash
go mod init winmove

go get github.com/BurntSushi/xgb
go get github.com/BurntSushi/xgb/randr
