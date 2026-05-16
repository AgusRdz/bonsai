package updater

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestIsDev(t *testing.T) {
	cases := []struct {
		version string
		want    bool
	}{
		{"dev", true},
		{"v1.0.0-dirty", true},
		{"v1.0.0", false},
		{"v0.1.2", false},
		{"v2.0.0", false},
	}
	for _, c := range cases {
		if got := IsDev(c.version); got != c.want {
			t.Errorf("IsDev(%q) = %v, want %v", c.version, got, c.want)
		}
	}
}

func TestParseSemver(t *testing.T) {
	cases := []struct {
		input string
		want  [3]int
		ok    bool
	}{
		{"v1.2.3", [3]int{1, 2, 3}, true},
		{"1.0.0", [3]int{1, 0, 0}, true},
		{"v0.0.1", [3]int{0, 0, 1}, true},
		{"v10.20.30", [3]int{10, 20, 30}, true},
		{"invalid", [3]int{}, false},
		{"1.2", [3]int{}, false},
		{"v1.2.x", [3]int{}, false},
		{"", [3]int{}, false},
	}
	for _, c := range cases {
		got, ok := parseSemver(c.input)
		if ok != c.ok {
			t.Errorf("parseSemver(%q) ok = %v, want %v", c.input, ok, c.ok)
			continue
		}
		if ok && got != c.want {
			t.Errorf("parseSemver(%q) = %v, want %v", c.input, got, c.want)
		}
	}
}

func TestIsNewer(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"v1.1.0", "v1.0.0", true},
		{"v1.0.1", "v1.0.0", true},
		{"v2.0.0", "v1.9.9", true},
		{"v1.0.0", "v1.0.0", false},
		{"v0.9.9", "v1.0.0", false},
		{"v1.0.0", "v1.1.0", false},
		{"v1.0.0", "v1.0.1", false},
		{"invalid", "v1.0.0", false},
		{"v1.0.0", "invalid", false},
	}
	for _, c := range cases {
		if got := isNewer(c.a, c.b); got != c.want {
			t.Errorf("isNewer(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestParseChecksum(t *testing.T) {
	checksums := "" +
		"abc123def456  bonsai-linux-amd64\n" +
		"111222333444  bonsai-darwin-arm64\n" +
		"aabbccddeeff *bonsai-windows-amd64.exe\n"

	cases := []struct {
		name    string
		binary  string
		want    string
		wantErr bool
	}{
		{"two-space format", "bonsai-linux-amd64", "abc123def456", false},
		{"second entry", "bonsai-darwin-arm64", "111222333444", false},
		{"asterisk format (Windows)", "bonsai-windows-amd64.exe", "aabbccddeeff", false},
		{"missing binary", "bonsai-linux-arm64", "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := parseChecksum(checksums, c.binary)
			if (err != nil) != c.wantErr {
				t.Fatalf("parseChecksum error = %v, wantErr %v", err, c.wantErr)
			}
			if !c.wantErr && got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestBuildBinaryName(t *testing.T) {
	name := buildBinaryName()
	if !strings.HasPrefix(name, "bonsai-") {
		t.Errorf("binary name %q does not start with bonsai-", name)
	}
	if runtime.GOOS == "windows" {
		if !strings.HasSuffix(name, ".exe") {
			t.Errorf("windows binary %q should end with .exe", name)
		}
	} else {
		if strings.HasSuffix(name, ".exe") {
			t.Errorf("non-windows binary %q should not end with .exe", name)
		}
	}
	// Format: bonsai-{os}-{arch}[.exe]
	parts := strings.Split(strings.TrimSuffix(name, ".exe"), "-")
	if len(parts) != 3 {
		t.Errorf("expected bonsai-os-arch format, got %q", name)
	}
}

func TestVerifySignatureWithKey(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	pubHex := hex.EncodeToString(pub)
	message := []byte("checksums content for testing")
	sig := ed25519.Sign(priv, message)
	sigHex := hex.EncodeToString(sig)

	t.Run("valid signature", func(t *testing.T) {
		if err := verifySignatureWithKey(pubHex, message, []byte(sigHex)); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("wrong message", func(t *testing.T) {
		if err := verifySignatureWithKey(pubHex, []byte("tampered"), []byte(sigHex)); err == nil {
			t.Error("expected error for wrong message, got nil")
		}
	})

	t.Run("corrupted signature", func(t *testing.T) {
		bad := strings.Repeat("00", ed25519.SignatureSize)
		if err := verifySignatureWithKey(pubHex, message, []byte(bad)); err == nil {
			t.Error("expected error for corrupted signature, got nil")
		}
	})

	t.Run("invalid hex signature", func(t *testing.T) {
		if err := verifySignatureWithKey(pubHex, message, []byte("not-hex")); err == nil {
			t.Error("expected error for invalid hex, got nil")
		}
	})

	t.Run("unconfigured key", func(t *testing.T) {
		if err := verifySignatureWithKey("REPLACE_WITH_OUTPUT_OF_MAKE_KEYGEN", message, []byte(sigHex)); err == nil {
			t.Error("expected error for unconfigured key, got nil")
		}
	})

	t.Run("wrong key", func(t *testing.T) {
		otherPub, _, _ := ed25519.GenerateKey(rand.Reader)
		if err := verifySignatureWithKey(hex.EncodeToString(otherPub), message, []byte(sigHex)); err == nil {
			t.Error("expected error for wrong public key, got nil")
		}
	})
}

func TestCheckBinaryMagic(t *testing.T) {
	write := func(b []byte) string {
		f, err := os.CreateTemp(t.TempDir(), "magic-*")
		if err != nil {
			t.Fatalf("CreateTemp: %v", err)
		}
		f.Write(b)
		f.Close()
		return f.Name()
	}

	var validMagic []byte
	switch runtime.GOOS {
	case "linux":
		validMagic = []byte{0x7f, 'E', 'L', 'F', 0x00}
	case "darwin":
		validMagic = []byte{0xcf, 0xfa, 0xed, 0xfe, 0x00}
	case "windows":
		validMagic = []byte{'M', 'Z', 0x00, 0x00, 0x00}
	}

	t.Run("valid magic bytes", func(t *testing.T) {
		if err := checkBinaryMagic(write(validMagic)); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("invalid magic bytes", func(t *testing.T) {
		if err := checkBinaryMagic(write([]byte{0x00, 0x00, 0x00, 0x00, 0x00})); err == nil {
			t.Error("expected error for invalid magic, got nil")
		}
	})

	t.Run("file too small", func(t *testing.T) {
		if err := checkBinaryMagic(write([]byte{0x7f})); err == nil {
			t.Error("expected error for file too small, got nil")
		}
	})
}
