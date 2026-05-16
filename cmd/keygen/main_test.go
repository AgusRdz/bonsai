package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInjectPublicKey(t *testing.T) {
	template := `package updater

const publicKey = "REPLACE_WITH_OUTPUT_OF_MAKE_KEYGEN"
`
	newKey := "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899"

	t.Run("injects key into placeholder", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "updater.go")
		if err := os.WriteFile(path, []byte(template), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		if err := injectPublicKey(path, newKey); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		content, _ := os.ReadFile(path)
		if !strings.Contains(string(content), `"`+newKey+`"`) {
			t.Errorf("key not injected, got:\n%s", content)
		}
		if strings.Contains(string(content), "REPLACE_WITH_OUTPUT_OF_MAKE_KEYGEN") {
			t.Error("placeholder still present after injection")
		}
	})

	t.Run("replaces existing key", func(t *testing.T) {
		existing := strings.ReplaceAll(template, "REPLACE_WITH_OUTPUT_OF_MAKE_KEYGEN",
			"0000000000000000000000000000000000000000000000000000000000000000")
		path := filepath.Join(t.TempDir(), "updater.go")
		if err := os.WriteFile(path, []byte(existing), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		if err := injectPublicKey(path, newKey); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		content, _ := os.ReadFile(path)
		if !strings.Contains(string(content), `"`+newKey+`"`) {
			t.Errorf("key not replaced, got:\n%s", content)
		}
	})

	t.Run("error on missing marker", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "updater.go")
		if err := os.WriteFile(path, []byte("package updater\n"), 0644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}

		if err := injectPublicKey(path, newKey); err == nil {
			t.Error("expected error for missing marker, got nil")
		}
	})

	t.Run("error on missing file", func(t *testing.T) {
		if err := injectPublicKey("/nonexistent/path.go", newKey); err == nil {
			t.Error("expected error for missing file, got nil")
		}
	})
}
