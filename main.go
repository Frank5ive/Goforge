package main

import (
	"embed"
	"fmt"
	"io/fs"
	"os"

	"github.com/Frank5ive/goforge/cmd"
)

// TemplatesFS holds the templates GoForge generates projects from, embedded in
// the binary so it needs no other files.
//
//go:embed templates
var TemplatesFS embed.FS

func main() {
	// Hand the embedded templates to the cmd package, rooted at templates/
	templates, err := fs.Sub(TemplatesFS, "templates")
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Error: %v\n", err)
		os.Exit(1)
	}
	cmd.SetTemplatesFS(templates)

	os.Exit(cmd.Execute())
}
