package usecase

import (
	"strings"
	"testing"

	"github.com/Frank5ive/goforge/internal/domain"
)

func TestNoDatabase(t *testing.T) {
	if !noDatabase(domain.Manifest{}) {
		t.Error("noDatabase() = false for a project without a database")
	}
	if noDatabase(domain.Manifest{Database: domain.DatabasePostgres}) {
		t.Error("noDatabase() = true for a project with PostgreSQL")
	}
}

func TestEngineFor(t *testing.T) {
	for _, name := range domain.DatabaseEngines {
		if e, err := engineFor(name); err != nil || e == nil {
			t.Errorf("engineFor(%q) = %v, %v", name, e, err)
		}
	}
	if _, err := engineFor("oracle"); err == nil || !strings.Contains(err.Error(), "oracle") {
		t.Errorf("engineFor(oracle) error = %v, want it to name the database", err)
	}
}

func TestFitLength(t *testing.T) {
	bound := func(v float64) *float64 { return &v }
	tests := []struct {
		name   string
		in     string
		field  domain.Field
		want   string
		length func(string) bool
	}{
		{"unbounded", "Gopher plush", domain.Field{}, "Gopher plush", nil},
		{"trimmed to the maximum", "Gopher plush", domain.Field{Max: bound(6)}, "Gopher", nil},
		{"padded to the minimum", "ab", domain.Field{Min: bound(5)}, "abxxx", nil},
		{"padding ignores surrounding spaces", "  ab  ", domain.Field{Min: bound(4)}, "abxx", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fitLength(tt.in, tt.field); got != tt.want {
				t.Errorf("fitLength(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
