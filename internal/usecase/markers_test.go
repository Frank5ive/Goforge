package usecase

import (
	"testing"
)

func TestInsertAboveMarker(t *testing.T) {
	src := "func f() {\n\ta()\n\t// marker\n}\n"

	got, err := insertAboveMarker(src, "// marker", "b()\nif x {\n\tc()\n}\n")
	if err != nil {
		t.Fatalf("insertAboveMarker() error = %v", err)
	}
	want := "func f() {\n\ta()\n\tb()\n\tif x {\n\t\tc()\n\t}\n\t// marker\n}\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}

	if _, err := insertAboveMarker(src, "// missing", "x"); err == nil {
		t.Error("expected an error for a missing marker")
	}
}

func TestAddImport(t *testing.T) {
	tests := []struct {
		name string
		src  string
		path string
		want string
	}{
		{
			name: "third party joins the last group",
			src:  "package p\n\nimport (\n\t\"fmt\"\n\n\t\"example.com/a\"\n)\n",
			path: "example.com/x",
			want: "package p\n\nimport (\n\t\"fmt\"\n\n\t\"example.com/a\"\n\t\"example.com/x\"\n)\n",
		},
		{
			name: "stdlib joins the first group",
			src:  "package p\n\nimport (\n\t\"fmt\"\n\n\t\"example.com/a\"\n)\n",
			path: "errors",
			want: "package p\n\nimport (\n\t\"fmt\"\n\t\"errors\"\n\n\t\"example.com/a\"\n)\n",
		},
		{
			name: "third party after stdlib-only block gets its own group",
			src:  "package p\n\nimport \"context\"\n\nfunc f() {}\n",
			path: "example.com/x",
			want: "package p\n\nimport (\n\t\"context\"\n\n\t\"example.com/x\"\n)\n\nfunc f() {}\n",
		},
		{
			name: "stdlib before third-party-only block gets its own group",
			src:  "package p\n\nimport (\n\t\"example.com/a\"\n)\n",
			path: "errors",
			want: "package p\n\nimport (\n\t\"errors\"\n\n\t\"example.com/a\"\n)\n",
		},
		{
			name: "already imported",
			src:  "package p\n\nimport \"example.com/x\"\n",
			path: "example.com/x",
			want: "package p\n\nimport \"example.com/x\"\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := addImport(tt.src, tt.path)
			if err != nil {
				t.Fatalf("addImport() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("got:\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}

	if _, err := addImport("package p\n", "example.com/x"); err == nil {
		t.Error("expected an error for a file with no imports")
	}
}
