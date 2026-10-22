package main

import (
	"embed"
	"fmt"
	"io/fs"
	"os"

	"github.com/Frank5ive/goforge/cmd"
)

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
