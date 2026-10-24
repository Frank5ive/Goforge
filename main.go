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
	os.Exit(run())
}

// run hands the embedded templates to the cmd package and runs goforge,
// returning the process's exit code.
func run() int {
	// The templates are rooted at templates/
	templates, err := fs.Sub(TemplatesFS, "templates")
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Error: %v\n", err)
		return 1
	}
	cmd.SetTemplatesFS(templates)

	return cmd.Execute()
}
