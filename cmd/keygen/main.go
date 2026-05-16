package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"os"
	"strings"
)

const updaterPath = "updater/updater.go"
const publicKeyMarker = `const publicKey = "`

func main() {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to generate key: %v\n", err)
		os.Exit(1)
	}

	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to marshal private key: %v\n", err)
		os.Exit(1)
	}
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privBytes})

	pubBytes, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to marshal public key: %v\n", err)
		os.Exit(1)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes})

	pubHex := hex.EncodeToString(pub)
	privB64 := base64.StdEncoding.EncodeToString(privPEM)

	if err := os.WriteFile("public_key.pem", pubPEM, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write public_key.pem: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile("private_key.pem", privPEM, 0600); err != nil {
		fmt.Fprintf(os.Stderr, "failed to write private_key.pem: %v\n", err)
		os.Exit(1)
	}

	if err := injectPublicKey(updaterPath, pubHex); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not update %s automatically: %v\n", updaterPath, err)
		fmt.Fprintf(os.Stderr, "set publicKey manually to: %s\n", pubHex)
	} else {
		fmt.Printf("public key injected into %s\n", updaterPath)
	}

	fmt.Println("public_key.pem  -> safe to commit")
	fmt.Println("private_key.pem -> keep this safe, do NOT commit (already in .gitignore)")
	fmt.Println()
	fmt.Println("GitHub Secret SIGNING_KEY (base64 of private_key.pem):")
	fmt.Println()
	fmt.Printf("  %s\n", privB64)
	fmt.Println()
	fmt.Println("IMPORTANT: back up private_key.pem. If lost, run 'make keygen' again and rotate the GitHub Secret.")
}

func injectPublicKey(path, pubHex string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	src := string(content)
	idx := strings.Index(src, publicKeyMarker)
	if idx == -1 {
		return fmt.Errorf("publicKey constant not found in %s", updaterPath)
	}

	start := idx + len(publicKeyMarker)
	end := strings.Index(src[start:], `"`)
	if end == -1 {
		return fmt.Errorf("malformed publicKey constant in %s", updaterPath)
	}

	updated := src[:start] + pubHex + src[start+end:]
	return os.WriteFile(path, []byte(updated), 0644)
}
