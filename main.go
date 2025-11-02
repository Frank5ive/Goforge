package main

import (
	"embed"
	"fmt"
	"os"

	"github.com/Frank5ive/goforge/cmd"
)

//go:embed templates/*.tmpl
var TemplatesFS embed.FS

func main() {
	// Set the embedded templates for cmd package to use
	cmd.SetTemplatesFS(TemplatesFS)
	
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
