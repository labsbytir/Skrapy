package metrics

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"golang.org/x/term"
)

// TUI implements the monospace-design-tui §11.1 Dashboard archetype:
// header metric cards, scrollable data table, footer key strip.
// Falls back to the line dashboard when stdout is not a TTY (§9 dual rendering).
type TUI struct {
	m        *Metrics
	out      *os.File
	oldState *term.State
	w, h     int
	filter   string
	sortCol  int // 0=url 1=status 2=ms 3=title
	sortDesc bool
	help     bool
	entering bool // filter input active
	fbuf     string
	quit     chan struct{}
	onQuit   func()
}

func colorOK() bool {
	t := os.Getenv("TERM")
	if t == "" || t == "dumb" || os.Getenv("NO_COLOR") != "" {
		return false
	}
	for _, a := range os.Args {
		if a == "--no-color" {
			return false
		}
	}
	return true
}

func (t *TUI) Run(m *Metrics, onQuit func()) error {
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		return fmt.Errorf("not a tty")
	}
	t.m = m
	t.out = os.Stdout
	t.quit = make(chan struct{})
	t.onQuit = onQuit
	fd := int(os.Stdin.Fd())
	old, err := term.MakeRaw(fd)
	if err != nil {
		return err
	}
	t.oldState = old
	defer term.Restore(fd, old)
	fmt.Fprint(t.out, "\033[?1049h\033[H")
	defer fmt.Fprint(t.out, "\033[?1049l")

	t.size()
	go t.input(fd)
	t.render()
	tick := 0
	for {
		select {
		case <-t.quit:
			return nil
		default:
		}
		tick++
		_ = tick
		t.render()
		waitKey(t.quit)
	}
}

func waitKey(quit chan struct{}) {
	// 200ms render cadence (§10 motion: responsive feedback).
	select {
	case <-quit:
	case <-time.After(200 * time.Millisecond):
	}
}

func (t *TUI) size() {
	if t.w != 0 && t.h != 0 {
		return // fixed size (tests, embedders)
	}
	w, h, err := term.GetSize(int(t.out.Fd()))
	if err != nil {
		w, h = 80, 24
	}
	t.w, t.h = w, h
}

func (t *TUI) input(fd int) {
	buf := make([]byte, 1)
	for {
		if _, err := os.Stdin.Read(buf); err != nil {
			return
		}
		c := buf[0]
		if t.entering {
			switch c {
			case 27: // Esc cancels
				t.entering = false
				t.fbuf = ""
			case 13: // Enter applies
				t.filter = t.fbuf
				t.entering = false
			case 127: // backspace
				if len(t.fbuf) > 0 {
					t.fbuf = t.fbuf[:len(t.fbuf)-1]
				}
			default:
				if c >= 32 && c < 127 {
					t.fbuf += string(c)
				}
			}
			continue
		}
		switch c {
		case 'q', 'Q', 3: // q or Ctrl-C
			close(t.quit)
			if t.onQuit != nil {
				t.onQuit()
			}
			return
		case '?':
			t.help = !t.help
		case 'r', 'R':
			// manual refresh: next render picks up fresh snapshot
		case '/':
			t.entering = true
			t.fbuf = t.filter
		case 's', 'S':
			t.sortDesc = !t.sortDesc
		case '1', '2', '3', '4':
			t.sortCol = int(c - '1')
		case 27:
			t.help = false
		}
	}
}

func (t *TUI) rows() []Row {
	s := t.m.Snap()
	rows := s.Rows
	if t.filter != "" {
		f := strings.ToLower(t.filter)
		kept := rows[:0:0]
		for _, r := range rows {
			if strings.Contains(strings.ToLower(r.URL+r.Title+r.Status), f) {
				kept = append(kept, r)
			}
		}
		rows = kept
	}
	col := t.sortCol
	sort.SliceStable(rows, func(i, j int) bool {
		var less bool
		switch col {
		case 1:
			less = rows[i].Status < rows[j].Status
		case 2:
			less = rows[i].MS < rows[j].MS
		case 3:
			less = rows[i].Title < rows[j].Title
		default:
			less = rows[i].URL < rows[j].URL
		}
		if t.sortDesc {
			return !less
		}
		return less
	})
	return rows
}

func (t *TUI) render() {
	t.size()
	w, h := t.w, t.h
	if w < 80 || h < 24 {
		fmt.Fprintf(t.out, "\033[H\033[2Jterminal too small (%dx%d), need 80x24", w, h)
		return
	}
	var b strings.Builder
	bar := strings.Repeat("═", w-2)
	b.WriteString("╔" + padCenter(" Skrapy ", w-2, "═") + "╗\n")
	s := t.m.Snap()
	cards := fmt.Sprintf(" ◉ OK: %d │ ◉ Fail: %d │ %.1f/s │ ⚡ Infer: %d calls %d tok │ ⬡ Proxy: %d ",
		s.PagesOK, s.PagesFail, s.PagesSec, s.InferCalls, s.InferTokens, s.ProxyRequests)
	if len(cards) > w-2 {
		cards = cards[:w-2]
	}
	b.WriteString("║" + padRight(cards, w-2) + "║\n")
	sp := " " + sparkline(s.Spark)
	if len(sp) > w-2 {
		sp = sp[:w-2]
	}
	b.WriteString("║" + padRight(sp, w-2) + "║\n")
	b.WriteString("╠" + bar + "╣\n")

	rows := t.rows()
	// header + cards(2) + table header + footer + borders
	avail := h - 7
	if t.help {
		avail -= 8
	}
	if t.entering {
		avail--
	}
	if avail < 1 {
		avail = 1
	}
	head := fmt.Sprintf(" %-40s │ %-4s │ %8s │ %s", "Source", "Stat", "MS", "Title")
	if len(head) > w-2 {
		head = head[:w-2]
	}
	b.WriteString("║" + padRight(head, w-2) + "║\n")
	start := 0
	if len(rows) > avail {
		start = len(rows) - avail
	}
	for _, r := range rows[start:] {
		g := glyph(r.Status)
		line := fmt.Sprintf(" %-40.40s │ %-4s │ %8d │ %.30s", r.URL, g, r.MS, r.Title)
		if len(line) > w-2 {
			line = line[:w-2]
		}
		b.WriteString("║" + padRight(line, w-2) + "║\n")
	}
	for i := len(rows[start:]); i < avail; i++ {
		b.WriteString("║" + strings.Repeat(" ", w-2) + "║\n")
	}
	b.WriteString("╠" + bar + "╣\n")
	foot := " ? Help  r Refresh  / Filter  s Sort  q Quit"
	if t.entering {
		foot = " / Filter: " + t.fbuf + "█  (Enter applies, Esc cancels)"
	} else if t.filter != "" {
		foot += "   [filter: " + t.filter + "]"
	}
	if len(foot) > w-2 {
		foot = foot[:w-2]
	}
	b.WriteString("║" + padRight(foot, w-2) + "║\n")
	b.WriteString("╚" + bar + "╝")
	if t.help {
		b.WriteString("\n" + helpBox(w))
	}
	fmt.Fprintf(t.out, "\033[H%s", b.String())
}

func glyph(status string) string {
	switch status {
	case "ok":
		return "◉ OK"
	case "slow":
		return "⚠ SL"
	default:
		return "✖ ER"
	}
}

func helpBox(w int) string {
	lines := []string{
		"? toggle help   r refresh   / filter   s sort asc/desc",
		"1-4 sort column (url status ms title)   q quit",
		"Esc closes help/filter",
	}
	var b strings.Builder
	bar := strings.Repeat("─", w-2)
	b.WriteString("┌" + bar + "┐\n")
	for _, l := range lines {
		if len(l) > w-2 {
			l = l[:w-2]
		}
		b.WriteString("│" + padRight(l, w-2) + "│\n")
	}
	b.WriteString("└" + bar + "┘")
	return b.String()
}

func padRight(s string, n int) string {
	if len(s) >= n {
		return s[:n]
	}
	return s + strings.Repeat(" ", n-len(s))
}

func padCenter(s string, n int, fill string) string {
	if len(s) >= n {
		return s[:n]
	}
	total := n - len(s)
	left := total / 2
	right := total - left
	var fl, fr strings.Builder
	for fl.Len() < left {
		fl.WriteString(fill)
	}
	for fr.Len() < right {
		fr.WriteString(fill)
	}
	return fl.String()[:left] + s + fr.String()[:right]
}
