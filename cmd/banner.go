package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"
)

// colorMode is the color capability of the output terminal.
type colorMode int

const (
	noColor colorMode = iota
	color256
	trueColor
)

type rgb struct{ r, g, b uint8 }

// gopherPixels is the pixel-art Go gopher. Each character is one pixel and maps to
// a color in gopherPalette; '.' is transparent. Two pixel rows share one terminal
// row via half-block characters, so 14 pixel rows render as 7 lines.
//
// The Go gopher was designed by Renée French and is licensed under CC BY 4.0.
var gopherPixels = []string{
	"..CC........CC..",
	".CCCCCCCCCCCCCC.",
	"CCWWWWCCCCWWWWCC",
	"CWWWWWWCCWWWWWWC",
	"CWWWKKWCCWKKWWWC",
	"CWWWKKWCCWKKWWWC",
	"CCWWWWCTTCWWWWCC",
	"CCCCCCTNNTCCCCCC",
	"CCCCCCTTTTCCCCCC",
	"CCCCCCCWWCCCCCCC",
	"TTCCCCCCCCCCCCTT",
	"CCCCCCCCCCCCCCCC",
	".CCCCCCCCCCCCCC.",
	"..TTT......TTT..",
}

var gopherPalette = map[byte]rgb{
	'C': {106, 215, 229}, // gopher blue
	'W': {255, 255, 255}, // eyes and teeth
	'K': {20, 20, 20},    // pupils
	'T': {246, 210, 162}, // snout, hands, feet
	'N': {75, 54, 33},    // nose
}

// wordmarkGo and wordmarkForge spell "GO" and "FORGE" in the ANSI Shadow figlet font.
var wordmarkGo = []string{
	" ██████╗  ██████╗ ",
	"██╔════╝ ██╔═══██╗",
	"██║  ███╗██║   ██║",
	"██║   ██║██║   ██║",
	"╚██████╔╝╚██████╔╝",
	" ╚═════╝  ╚═════╝ ",
}

var wordmarkForge = []string{
	"███████╗ ██████╗ ██████╗  ██████╗ ███████╗",
	"██╔════╝██╔═══██╗██╔══██╗██╔════╝ ██╔════╝",
	"█████╗  ██║   ██║██████╔╝██║  ███╗█████╗  ",
	"██╔══╝  ██║   ██║██╔══██╗██║   ██║██╔══╝  ",
	"██║     ╚██████╔╝██║  ██║╚██████╔╝███████╗",
	"╚═╝      ╚═════╝ ╚═╝  ╚═╝ ╚═════╝ ╚══════╝",
}

var (
	goBlue     = rgb{0, 173, 216}
	emberStart = rgb{255, 78, 0}
	emberEnd   = rgb{255, 179, 0}
	mutedGray  = rgb{138, 138, 138}
)

const (
	tagline = "Forge production-ready Go servers in seconds"
	// gopherGap is the space between the gopher and the text beside it
	gopherGap = "   "
)

// printBanner writes the GoForge banner to w when w is an interactive terminal.
// Piped or redirected output gets no banner, so scripts and CI logs stay clean.
func printBanner(w io.Writer) {
	f, ok := w.(*os.File)
	if !ok || !term.IsTerminal(int(f.Fd())) {
		return
	}

	width := 80
	if cols, _, err := term.GetSize(int(f.Fd())); err == nil && cols > 0 {
		width = cols
	}

	fmt.Fprint(w, renderBanner(detectColorMode(), width))
}

// detectColorMode follows the NO_COLOR convention (https://no-color.org) and
// COLORTERM for truecolor support, falling back to 256 colors.
func detectColorMode() colorMode {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return noColor
	}

	switch strings.ToLower(os.Getenv("COLORTERM")) {
	case "truecolor", "24bit":
		return trueColor
	}
	if os.Getenv("WT_SESSION") != "" { // Windows Terminal
		return trueColor
	}

	return color256
}

// renderBanner builds the banner for the given color mode and terminal width.
// Wide color terminals get the gopher beside the full wordmark; narrower ones get
// the gopher beside a compact title. Without color, or when even the compact
// layout does not fit, the gopher is omitted.
func renderBanner(mode colorMode, width int) string {
	gopherWidth := len(gopherPixels[0]) + len(gopherGap)
	wordmarkWidth := runeWidth(wordmarkGo[0]) + runeWidth(wordmarkForge[0])
	showGopher := mode != noColor && width >= gopherWidth+runeWidth(plainTitle())

	var text []string
	switch {
	case mode == noColor && width >= wordmarkWidth:
		text = renderWordmark(mode)
	case !showGopher:
		text = []string{renderTitle(mode)}
	case width >= gopherWidth+wordmarkWidth:
		text = renderWordmark(mode)
	default:
		text = renderCompactTitle(mode, width-gopherWidth)
	}

	var b strings.Builder
	b.WriteString("\n")

	if !showGopher {
		for _, line := range text {
			b.WriteString(line + "\n")
		}
		b.WriteString("\n")
		return b.String()
	}

	gopher := renderGopher(mode)
	for i, line := range gopher {
		b.WriteString(line)
		if i < len(text) {
			b.WriteString(gopherGap + text[i])
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")

	return b.String()
}

// renderWordmark returns the "GOFORGE" wordmark followed by the tagline. Letter
// strokes are full color and the drop shadow is a darker shade of the same color.
func renderWordmark(mode colorMode) []string {
	forgeWidth := runeWidth(wordmarkForge[0])
	lines := make([]string, 0, len(wordmarkGo)+1)

	for row := range wordmarkGo {
		var b strings.Builder

		for _, r := range wordmarkGo[row] {
			b.WriteString(paintGlyph(mode, r, goBlue))
		}

		for col, r := range []rune(wordmarkForge[row]) {
			t := float64(col) / float64(forgeWidth-1)
			b.WriteString(paintGlyph(mode, r, lerp(emberStart, emberEnd, t)))
		}

		lines = append(lines, b.String())
	}

	return append(lines, " "+paint(mode, mutedGray, tagline+" · v"+version))
}

// renderCompactTitle is used beside the gopher when the terminal is too narrow for
// the wordmark. The tagline is included only if it fits in the available width.
func renderCompactTitle(mode colorMode, available int) []string {
	lines := []string{"", "", renderTitle(mode)}
	if runeWidth(tagline) <= available {
		lines = append(lines, paint(mode, mutedGray, tagline))
	}
	return lines
}

// renderTitle returns "GoForge vX.Y.Z" with "Go" in Go blue and "Forge" in ember.
func renderTitle(mode colorMode) string {
	return bold(mode, paint(mode, goBlue, "Go")+paint(mode, emberStart, "Forge")) + " " + paint(mode, mutedGray, "v"+version)
}

func plainTitle() string {
	return "GoForge v" + version
}

// renderGopher converts the gopher pixels into terminal lines. Each line holds two
// pixel rows: '▀' with the top pixel as foreground and the bottom as background.
func renderGopher(mode colorMode) []string {
	lines := make([]string, 0, (len(gopherPixels)+1)/2)

	for row := 0; row < len(gopherPixels); row += 2 {
		top := gopherPixels[row]
		bottom := strings.Repeat(".", len(top))
		if row+1 < len(gopherPixels) {
			bottom = gopherPixels[row+1]
		}

		var b strings.Builder
		for col := 0; col < len(top); col++ {
			topColor, hasTop := gopherPalette[top[col]]
			bottomColor, hasBottom := gopherPalette[bottom[col]]

			switch {
			case hasTop && hasBottom:
				b.WriteString(fg(mode, topColor) + bg(mode, bottomColor) + "▀" + reset)
			case hasTop:
				b.WriteString(fg(mode, topColor) + "▀" + reset)
			case hasBottom:
				b.WriteString(fg(mode, bottomColor) + "▄" + reset)
			default:
				b.WriteString(" ")
			}
		}

		lines = append(lines, b.String())
	}

	return lines
}

// paintGlyph colors a wordmark character: solid blocks get the full color and the
// box-drawing shadow gets a darker shade.
func paintGlyph(mode colorMode, r rune, c rgb) string {
	switch r {
	case ' ':
		return " "
	case '█':
		return paint(mode, c, string(r))
	default:
		return paint(mode, shade(c, 0.55), string(r))
	}
}

const reset = "\x1b[0m"

func paint(mode colorMode, c rgb, s string) string {
	if mode == noColor {
		return s
	}
	return fg(mode, c) + s + reset
}

func bold(mode colorMode, s string) string {
	if mode == noColor {
		return s
	}
	return "\x1b[1m" + s + reset
}

func fg(mode colorMode, c rgb) string {
	if mode == trueColor {
		return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", c.r, c.g, c.b)
	}
	return fmt.Sprintf("\x1b[38;5;%dm", to256(c))
}

func bg(mode colorMode, c rgb) string {
	if mode == trueColor {
		return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", c.r, c.g, c.b)
	}
	return fmt.Sprintf("\x1b[48;5;%dm", to256(c))
}

// to256 maps a color to the nearest entry in the xterm 6x6x6 color cube.
func to256(c rgb) int {
	levels := []int{0, 95, 135, 175, 215, 255}
	nearest := func(v uint8) int {
		best := 0
		for i, level := range levels {
			if abs(int(v)-level) < abs(int(v)-levels[best]) {
				best = i
			}
		}
		return best
	}
	return 16 + 36*nearest(c.r) + 6*nearest(c.g) + nearest(c.b)
}

func lerp(a, b rgb, t float64) rgb {
	mix := func(x, y uint8) uint8 { return uint8(float64(x) + (float64(y)-float64(x))*t) }
	return rgb{mix(a.r, b.r), mix(a.g, b.g), mix(a.b, b.b)}
}

func shade(c rgb, factor float64) rgb {
	return rgb{uint8(float64(c.r) * factor), uint8(float64(c.g) * factor), uint8(float64(c.b) * factor)}
}

func runeWidth(s string) int {
	return len([]rune(s))
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
