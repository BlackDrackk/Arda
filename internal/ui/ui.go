// Package ui provides terminal output utilities for the Arda CLI.
// It respects the NO_COLOR convention (https://no-color.org) and TERM=dumb.
package ui

import (
	"fmt"
	"os"
	"strings"
)

// ANSI escape sequences.
const (
	reset = "\033[0m"
	bold  = "\033[1m"
	dim   = "\033[2m"

	red     = "\033[31m"
	green   = "\033[32m"
	yellow  = "\033[33m"
	blue    = "\033[34m"
	magenta = "\033[35m"
	cyan    = "\033[36m"
)

// NoColor reports whether ANSI output should be suppressed.
// Respects NO_COLOR (https://no-color.org) and TERM=dumb.
func NoColor() bool {
	_, disabled := os.LookupEnv("NO_COLOR")
	return disabled || os.Getenv("TERM") == "dumb"
}

func colorize(code, s string) string {
	if NoColor() {
		return s
	}
	return code + s + reset
}

// Success prints a green [+] line.
func Success(format string, a ...any) {
	tag := colorize(green+bold, "[+]")
	fmt.Printf(tag+" "+format+"\n", a...)
}

// Fail prints a red [-] line.
func Fail(format string, a ...any) {
	tag := colorize(red+bold, "[-]")
	fmt.Printf(tag+" "+format+"\n", a...)
}

// Info prints a blue [~] line.
func Info(format string, a ...any) {
	tag := colorize(blue+bold, "[~]")
	fmt.Printf(tag+" "+format+"\n", a...)
}

// Warn prints a yellow [!] line.
func Warn(format string, a ...any) {
	tag := colorize(yellow+bold, "[!]")
	fmt.Printf(tag+" "+format+"\n", a...)
}

// StateColor wraps a container state string in the appropriate ANSI color.
func StateColor(state string) string {
	if NoColor() {
		return state
	}
	switch strings.ToLower(state) {
	case "running":
		return green + bold + state + reset
	case "stopped", "exited":
		return red + state + reset
	case "paused":
		return yellow + state + reset
	case "created":
		return cyan + state + reset
	default:
		return dim + state + reset
	}
}

// ── Table ─────────────────────────────────────────────────────────────────────

// Table is a minimal bordered table renderer that handles ANSI-colored cells.
type Table struct {
	headers []string
	rows    [][]string
	widths  []int
}

// NewTable creates a Table with the given column headers.
func NewTable(headers ...string) *Table {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	return &Table{headers: headers, widths: widths}
}

// AddRow appends a data row, growing column widths as needed.
// Cells may already contain ANSI sequences; visible width is computed correctly.
func (t *Table) AddRow(cells ...string) {
	for i, c := range cells {
		if i >= len(t.widths) {
			break
		}
		if w := visibleLen(c); w > t.widths[i] {
			t.widths[i] = w
		}
	}
	t.rows = append(t.rows, cells)
}

// Render prints the complete table to stdout.
func (t *Table) Render() {
	fmt.Println(t.border("╭", "┬", "╮"))
	t.printRow(t.headers, true)
	fmt.Println(t.border("├", "┼", "┤"))
	for _, row := range t.rows {
		t.printRow(row, false)
	}
	fmt.Println(t.border("╰", "┴", "╯"))
}

func (t *Table) border(l, m, r string) string {
	var sb strings.Builder
	if !NoColor() {
		sb.WriteString(dim)
	}
	sb.WriteString(l)
	for i, w := range t.widths {
		sb.WriteString(strings.Repeat("─", w+2))
		if i < len(t.widths)-1 {
			sb.WriteString(m)
		}
	}
	sb.WriteString(r)
	if !NoColor() {
		sb.WriteString(reset)
	}
	return sb.String()
}

func (t *Table) printRow(cells []string, header bool) {
	noColor := NoColor()

	pipe := "│"
	if !noColor {
		pipe = dim + "│" + reset
	}

	fmt.Print(pipe)
	for i, cell := range cells {
		if i >= len(t.widths) {
			break
		}
		pad := strings.Repeat(" ", t.widths[i]-visibleLen(cell))
		if header && !noColor {
			fmt.Printf(" "+bold+cyan+"%s"+reset+"%s %s", cell, pad, pipe)
		} else {
			fmt.Printf(" %s%s %s", cell, pad, pipe)
		}
	}
	fmt.Println()
}

// visibleLen returns the number of printable characters in s,
// ignoring ANSI CSI escape sequences.
func visibleLen(s string) int {
	n, inEsc := 0, false
	for _, r := range s {
		switch {
		case r == '\033':
			inEsc = true
		case inEsc && r == 'm':
			inEsc = false
		case !inEsc:
			n++
		}
	}
	return n
}
