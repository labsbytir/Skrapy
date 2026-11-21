package metrics

import (
	"fmt"
	"strings"
)

// Tir palette (spec: hex lives in CSS, terminals get 24-bit ANSI).
type RGB struct{ R, G, B int }

var (
	Blue    = RGB{37, 110, 255}  // #256EFF digital-blue: header, links, focus
	Indigo  = RGB{70, 35, 122}   // #46237A indigo-velvet: background
	Emerald = RGB{61, 220, 151}  // #3DDC97 emerald: success / 2xx
	White   = RGB{252, 252, 252} // #FCFCFC white: primary text
	Fuchsia = RGB{255, 73, 92}   // #FF495C hot-fuchsia: errors
)

func fg(c RGB) string { return fmt.Sprintf("\033[38;2;%d;%d;%dm", c.R, c.G, c.B) }
func bg(c RGB) string { return fmt.Sprintf("\033[48;2;%d;%d;%dm", c.R, c.G, c.B) }

const ansiReset = "\033[0m"

// theme wraps s in color when the terminal supports it.
func theme(s string, c RGB) string {
	if !colorOK() {
		return s
	}
	return fg(c) + s + ansiReset
}

// statusGlyph pairs color with a status-code text (never color-only, §5/§7).
func statusGlyph(status string) string {
	switch status {
	case "ok":
		return theme("◉ OK", Emerald)
	case "slow":
		return theme("⚠ SL", Blue)
	case "robots":
		return theme("◉ RB", Blue)
	default:
		return theme("✖ ER", Fuchsia)
	}
}

// Bar is a zero-dep monospace bar: `label  25% ██████...` (width blocks).
func Bar(label string, pct float64, width int) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	n := int(pct/100*float64(width) + 0.5)
	return fmt.Sprintf("%-10s %3.0f%% %s%s", label, pct,
		strings.Repeat("█", n), strings.Repeat("░", width-n))
}
