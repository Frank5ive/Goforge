package cmd

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var updateLogo = flag.Bool("update-logo", false, "regenerate docs/logo.svg from the banner art")

// logoPath is the README logo, generated from the same gopher pixels and wordmark
// as the terminal banner so the two never drift apart.
var logoPath = filepath.Join("..", "docs", "logo.svg")

// TestLogoSVGUpToDate fails when docs/logo.svg does not match the banner art.
// Regenerate it with: go test ./cmd -run TestLogoSVGUpToDate -update-logo
func TestLogoSVGUpToDate(t *testing.T) {
	want := renderLogoSVG()

	if *updateLogo {
		if err := os.MkdirAll(filepath.Dir(logoPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(logoPath, []byte(want), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}

	got, err := os.ReadFile(logoPath)
	if err != nil {
		t.Fatalf("reading %s: %v (generate it with -update-logo)", logoPath, err)
	}
	if string(got) != want {
		t.Errorf("%s is out of date with the banner art; regenerate it with:\n  go test ./cmd -run TestLogoSVGUpToDate -update-logo", logoPath)
	}
}

// Logo geometry, in SVG pixels. A terminal cell is twice as tall as it is wide,
// so one gopher pixel (half a cell) is square.
const (
	cellW     = 10
	cellH     = 20
	pixelSize = cellW
	padX      = 32
	titleBarH = 36
	padTop    = 22
	padBottom = 26
)

var (
	windowBg     = rgb{22, 24, 30}
	windowBorder = rgb{48, 54, 61}
	titleBarBg   = rgb{30, 33, 40}
	promptGreen  = rgb{80, 200, 120}
)

// renderLogoSVG draws the banner as a dark terminal window, so it looks the same
// on light and dark GitHub themes.
func renderLogoSVG() string {
	gopherCols := len(gopherPixels[0])
	gapCols := len(gopherGap)
	wordmarkCols := runeWidth(wordmarkGo[0]) + runeWidth(wordmarkForge[0])
	bannerRows := (len(gopherPixels) + 1) / 2

	contentW := (gopherCols + gapCols + wordmarkCols) * cellW
	width := contentW + 2*padX

	promptY := titleBarH + padTop
	bannerY := promptY + cellH + cellH/2
	height := bannerY + bannerRows*cellH + padBottom

	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d" role="img" aria-labelledby="title">`+"\n", width, height, width, height)
	b.WriteString(`  <title id="title">GoForge: a pixel-art Go gopher next to the GOFORGE wordmark</title>` + "\n")

	// Window and title bar
	fmt.Fprintf(&b, `  <rect x="0.5" y="0.5" width="%d" height="%d" rx="12" fill="%s" stroke="%s"/>`+"\n", width-1, height-1, hex(windowBg), hex(windowBorder))
	fmt.Fprintf(&b, `  <path d="M0.5 %d V12.5 A12 12 0 0 1 12.5 0.5 H%d A12 12 0 0 1 %d 12.5 V%d Z" fill="%s"/>`+"\n",
		titleBarH, width-13, width-1, titleBarH, hex(titleBarBg))
	fmt.Fprintf(&b, `  <line x1="0.5" y1="%d" x2="%d" y2="%d" stroke="%s"/>`+"\n", titleBarH, width-1, titleBarH, hex(windowBorder))
	for i, c := range []string{"#ff5f57", "#febc2e", "#28c840"} {
		fmt.Fprintf(&b, `  <circle cx="%d" cy="%d" r="6" fill="%s"/>`+"\n", 22+i*20, titleBarH/2, c)
	}
	fmt.Fprintf(&b, `  <text x="%d" y="%d" fill="%s" font-family="ui-monospace, SFMono-Regular, Menlo, Consolas, monospace" font-size="13" text-anchor="middle">goforge</text>`+"\n",
		width/2, titleBarH/2+4, hex(mutedGray))

	// Prompt line
	fmt.Fprintf(&b, `  <text x="%d" y="%d" font-family="ui-monospace, SFMono-Regular, Menlo, Consolas, monospace" font-size="15"><tspan fill="%s">$</tspan><tspan fill="#e6edf3"> goforge</tspan></text>`+"\n",
		padX, promptY+cellH-5, hex(promptGreen))

	// Gopher: one rect per horizontal run of same-colored pixels
	b.WriteString(`  <g shape-rendering="crispEdges">` + "\n")
	for y, row := range gopherPixels {
		for x := 0; x < len(row); {
			c, ok := gopherPalette[row[x]]
			run := 1
			for x+run < len(row) && row[x+run] == row[x] {
				run++
			}
			if ok {
				fmt.Fprintf(&b, `    <rect x="%d" y="%d" width="%d" height="%d" fill="%s"/>`+"\n",
					padX+x*pixelSize, bannerY+y*pixelSize, run*pixelSize, pixelSize, hex(c))
			}
			x += run
		}
	}
	b.WriteString("  </g>\n")

	// Wordmark, starting one row down to match the terminal layout
	wordmarkX := padX + (gopherCols+gapCols)*cellW
	wordmarkY := bannerY + cellH
	forgeCols := runeWidth(wordmarkForge[0])

	b.WriteString(`  <g shape-rendering="crispEdges" stroke-width="1.6" fill="none">` + "\n")
	for row := range wordmarkGo {
		y := wordmarkY + row*cellH
		for col, r := range []rune(wordmarkGo[row]) {
			writeGlyph(&b, r, wordmarkX+col*cellW, y, goBlue)
		}
		forgeX := wordmarkX + runeWidth(wordmarkGo[0])*cellW
		for col, r := range []rune(wordmarkForge[row]) {
			t := float64(col) / float64(forgeCols-1)
			writeGlyph(&b, r, forgeX+col*cellW, y, lerp(emberStart, emberEnd, t))
		}
	}
	b.WriteString("  </g>\n")

	// Tagline beside the gopher's feet
	fmt.Fprintf(&b, `  <text x="%d" y="%d" fill="%s" font-family="ui-monospace, SFMono-Regular, Menlo, Consolas, monospace" font-size="15">%s</text>`+"\n",
		wordmarkX+cellW, wordmarkY+len(wordmarkGo)*cellH+cellH-5, hex(mutedGray), tagline)

	b.WriteString("</svg>\n")
	return b.String()
}

// writeGlyph draws one wordmark character in a cell whose top-left corner is (x, y).
// Full blocks are filled rects; box-drawing characters are drawn as the double
// lines a terminal would show, in a darker shade to form the drop shadow.
func writeGlyph(b *strings.Builder, r rune, x, y int, c rgb) {
	if r == '█' {
		fmt.Fprintf(b, `    <rect x="%d" y="%d" width="%d" height="%d" fill="%s" stroke="none"/>`+"\n", x, y, cellW, cellH, hex(c))
		return
	}

	// Double-line positions within the cell
	l, rr := x+3, x+7         // vertical strokes
	t, bm := y+8, y+12        // horizontal strokes
	left, right := x, x+cellW // cell edges
	top, bottom := y, y+cellH

	var d string
	switch r {
	case '═':
		d = fmt.Sprintf("M%d %d H%d M%d %d H%d", left, t, right, left, bm, right)
	case '║':
		d = fmt.Sprintf("M%d %d V%d M%d %d V%d", l, top, bottom, rr, top, bottom)
	case '╗':
		d = fmt.Sprintf("M%d %d H%d V%d M%d %d H%d V%d", left, t, rr, bottom, left, bm, l, bottom)
	case '╝':
		d = fmt.Sprintf("M%d %d H%d V%d M%d %d H%d V%d", left, bm, rr, top, left, t, l, top)
	case '╚':
		d = fmt.Sprintf("M%d %d V%d H%d M%d %d V%d H%d", l, top, bm, right, rr, top, t, right)
	case '╔':
		d = fmt.Sprintf("M%d %d V%d H%d M%d %d V%d H%d", l, bottom, t, right, rr, bottom, bm, right)
	default:
		return
	}
	fmt.Fprintf(b, `    <path d="%s" stroke="%s"/>`+"\n", d, hex(shade(c, 0.55)))
}

func hex(c rgb) string {
	return fmt.Sprintf("#%02x%02x%02x", c.r, c.g, c.b)
}

func TestWordmarkUsesOnlyDrawableGlyphs(t *testing.T) {
	drawable := "█═║╗╝╚╔ "
	for _, letters := range [][]string{wordmarkGo, wordmarkForge} {
		for _, row := range letters {
			for _, r := range row {
				if !strings.ContainsRune(drawable, r) {
					t.Errorf("wordmark uses %q, which renderLogoSVG cannot draw", r)
				}
			}
		}
	}
}
