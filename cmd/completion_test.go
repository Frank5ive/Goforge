package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestCompletionIsEnabled(t *testing.T) {
	if rootCmd.CompletionOptions.DisableDefaultCmd {
		t.Fatal("the `goforge completion` command is disabled")
	}

	generators := map[string]func(*bytes.Buffer) error{
		"bash":       func(b *bytes.Buffer) error { return rootCmd.GenBashCompletionV2(b, true) },
		"zsh":        func(b *bytes.Buffer) error { return rootCmd.GenZshCompletion(b) },
		"fish":       func(b *bytes.Buffer) error { return rootCmd.GenFishCompletion(b, true) },
		"powershell": func(b *bytes.Buffer) error { return rootCmd.GenPowerShellCompletionWithDesc(b) },
	}

	for shell, generate := range generators {
		t.Run(shell, func(t *testing.T) {
			var out bytes.Buffer
			if err := generate(&out); err != nil {
				t.Fatalf("generating %s completion: %v", shell, err)
			}
			if !strings.Contains(out.String(), "goforge") {
				t.Errorf("%s completion script does not mention goforge", shell)
			}
		})
	}
}
