package cmd

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
)

var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// visibleLines strips ANSI escapes and returns the non-empty banner lines.
func visibleLines(banner string) []string {
	var lines []string
	for _, line := range strings.Split(ansiEscape.ReplaceAllString(banner, ""), "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func TestRenderBanner(t *testing.T) {
	tests := []struct {
		name         string
		mode         colorMode
		width        int
		wantContains string
		wantColor    bool
		wantLines    int
	}{
		{"truecolor wide shows gopher and wordmark", trueColor, 120, "██████╗", true, 7},
		{"256 color fits 80 columns", color256, 80, "██████╗", true, 7},
		{"color medium uses compact title with tagline", trueColor, 70, "in seconds", true, 7},
		{"color narrow drops tagline", trueColor, 60, "GoForge " + displayVersion(), true, 7},
		{"color very narrow drops gopher", color256, 25, "GoForge " + displayVersion(), true, 1},
		{"no color wide shows plain wordmark and tagline", noColor, 120, "in seconds", false, 7},
		{"no color narrow shows plain title", noColor, 40, "GoForge " + displayVersion(), false, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			banner := renderBanner(tt.mode, tt.width)

			if hasColor := strings.Contains(banner, "\x1b["); hasColor != tt.wantColor {
				t.Errorf("color escapes present = %t, want %t", hasColor, tt.wantColor)
			}

			lines := visibleLines(banner)
			if len(lines) != tt.wantLines {
				t.Errorf("got %d visible lines, want %d:\n%s", len(lines), tt.wantLines, strings.Join(lines, "\n"))
			}

			for _, line := range lines {
				if w := runeWidth(strings.TrimRight(line, " ")); w > tt.width {
					t.Errorf("line is %d columns wide, terminal is %d: %q", w, tt.width, line)
				}
			}

			if !strings.Contains(ansiEscape.ReplaceAllString(banner, ""), tt.wantContains) {
				t.Errorf("banner does not contain %q", tt.wantContains)
			}
		})
	}
}

func TestRenderBanner_LongVersionFits(t *testing.T) {
	original := version
	version = "0.0.0-20261021193000-abcdef123456+dirty"
	t.Cleanup(func() { version = original })

	for _, line := range visibleLines(renderBanner(trueColor, 80)) {
		if w := runeWidth(strings.TrimRight(line, " ")); w > 80 {
			t.Errorf("line is %d columns wide with a long version: %q", w, line)
		}
	}
}

func TestDisplayVersion(t *testing.T) {
	original := version
	t.Cleanup(func() { version = original })

	for value, want := range map[string]string{"0.1.0": "v0.1.0", "dev": "dev"} {
		version = value
		if got := displayVersion(); got != want {
			t.Errorf("displayVersion() with version %q = %q, want %q", value, got, want)
		}
	}
}

func TestGopherPixelsAreConsistent(t *testing.T) {
	width := len(gopherPixels[0])
	for i, row := range gopherPixels {
		if len(row) != width {
			t.Errorf("row %d has %d pixels, want %d", i, len(row), width)
		}
		for _, p := range []byte(row) {
			if _, ok := gopherPalette[p]; !ok && p != '.' {
				t.Errorf("row %d uses %q, which is not in the palette", i, p)
			}
		}
	}
}

func TestWordmarkRowsAreAligned(t *testing.T) {
	for _, letters := range [][]string{wordmarkGo, wordmarkForge} {
		width := runeWidth(letters[0])
		for i, row := range letters {
			if runeWidth(row) != width {
				t.Errorf("wordmark row %d is %d wide, want %d: %q", i, runeWidth(row), width, row)
			}
		}
	}
}

func TestPrintBannerSkipsNonTerminal(t *testing.T) {
	var buf bytes.Buffer
	printBanner(&buf)

	if buf.Len() != 0 {
		t.Errorf("expected no banner for non-terminal output, got %q", buf.String())
	}
}

func TestTo256(t *testing.T) {
	tests := []struct {
		color rgb
		want  int
	}{
		{rgb{0, 0, 0}, 16},
		{rgb{255, 255, 255}, 231},
		{rgb{255, 0, 0}, 196},
	}

	for _, tt := range tests {
		if got := to256(tt.color); got != tt.want {
			t.Errorf("to256(%v) = %d, want %d", tt.color, got, tt.want)
		}
	}
}

func TestDetectColorModeRespectsNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	t.Setenv("COLORTERM", "truecolor")

	if got := detectColorMode(); got != noColor {
		t.Errorf("detectColorMode() = %v, want noColor", got)
	}
}
