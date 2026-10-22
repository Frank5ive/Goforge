package usecase

import (
	"errors"
	"fmt"
	"go/format"
	"strconv"
	"strings"

	"github.com/Frank5ive/goforge/internal/domain"
)

// insertAboveMarker inserts code on the lines just above marker, indented like the
// marker, so repeated insertions stack in order. code may span several lines.
func insertAboveMarker(src, marker, code string) (string, error) {
	at := strings.Index(src, marker)
	if at < 0 {
		return "", domain.WithCode(domain.CodeMarkerMissing, fmt.Errorf("the %q marker is missing", marker))
	}

	lineStart := strings.LastIndex(src[:at], "\n") + 1
	indent := src[lineStart:at]

	var b strings.Builder
	for _, line := range strings.Split(strings.TrimRight(code, "\n"), "\n") {
		if line == "" {
			b.WriteString("\n")
			continue
		}
		b.WriteString(indent + line + "\n")
	}

	return src[:lineStart] + b.String() + src[lineStart:], nil
}

// addImport adds importPath to src's imports, following goimports grouping:
// standard library imports go in the first group and others in the last, with a
// new group created when needed. gofmt then sorts each group. It is a no-op if
// the path is already imported.
func addImport(src, importPath string) (string, error) {
	quoted := strconv.Quote(importPath)
	if strings.Contains(src, quoted) {
		return src, nil
	}

	src, err := ensureImportBlock(src)
	if err != nil {
		return "", err
	}

	start := strings.Index(src, "import (\n") + len("import (\n")
	end := strings.Index(src[start:], ")")
	if end < 0 {
		return "", errors.New("unterminated import block")
	}
	end += start

	// Split the block into groups of import lines separated by blank lines
	var groups [][]string
	current := []string{}
	for _, line := range strings.Split(strings.TrimRight(src[start:end], "\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			if len(current) > 0 {
				groups = append(groups, current)
				current = []string{}
			}
			continue
		}
		current = append(current, line)
	}
	if len(current) > 0 {
		groups = append(groups, current)
	}

	line := "\t" + quoted
	std := isStdlib(importPath)
	switch {
	case len(groups) == 0:
		groups = [][]string{{line}}
	case std && groupIsStdlib(groups[0]):
		groups[0] = append(groups[0], line)
	case std:
		groups = append([][]string{{line}}, groups...)
	case !groupIsStdlib(groups[len(groups)-1]):
		groups[len(groups)-1] = append(groups[len(groups)-1], line)
	default:
		groups = append(groups, []string{line})
	}

	rendered := make([]string, len(groups))
	for i, g := range groups {
		rendered[i] = strings.Join(g, "\n")
	}
	return src[:start] + strings.Join(rendered, "\n\n") + "\n" + src[end:], nil
}

// ensureImportBlock turns a single `import "x"` into an import block.
func ensureImportBlock(src string) (string, error) {
	if strings.Contains(src, "import (\n") {
		return src, nil
	}

	single := strings.Index(src, "import \"")
	if single < 0 {
		return "", errors.New("no imports found")
	}
	lineEnd := strings.Index(src[single:], "\n")
	if lineEnd < 0 {
		return "", errors.New("unterminated import")
	}
	lineEnd += single

	existing := strings.TrimSpace(strings.TrimPrefix(src[single:lineEnd], "import"))
	return src[:single] + "import (\n\t" + existing + "\n)" + src[lineEnd:], nil
}

// isStdlib reports whether importPath is in the standard library, which by
// convention has no dot in its first path element.
func isStdlib(importPath string) bool {
	first, _, _ := strings.Cut(importPath, "/")
	return !strings.Contains(first, ".")
}

// groupIsStdlib reports whether a group of import lines holds standard library imports.
func groupIsStdlib(group []string) bool {
	path, err := strconv.Unquote(strings.Fields(group[0])[len(strings.Fields(group[0]))-1])
	return err == nil && isStdlib(path)
}

// formatGo runs gofmt on Go source produced by marker edits.
func formatGo(name, src string) (string, error) {
	formatted, err := format.Source([]byte(src))
	if err != nil {
		return "", fmt.Errorf("updated %s is not valid Go: %w", name, err)
	}
	return string(formatted), nil
}
