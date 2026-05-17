// Package auth manages OAuth tokens for bonsai's direct API providers.
// Tokens are stored in ~/.bonsai.tokens (TOML, chmod 600).
package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
)

// Token holds an OAuth access token for one hosting provider.
type Token struct {
	Host        string    `toml:"host"`
	AccessToken string    `toml:"access_token"`
	TokenType   string    `toml:"token_type"`
	Scopes      []string  `toml:"scopes"`
	ExpiresAt   time.Time `toml:"expires_at"`
	SavedAt     time.Time `toml:"saved_at"`
}

// Valid reports whether the token is present and not expired.
func (t Token) Valid() bool {
	if t.AccessToken == "" {
		return false
	}
	if t.ExpiresAt.IsZero() {
		return true
	}
	return time.Now().Before(t.ExpiresAt)
}

type tokenFile struct {
	Tokens map[string]encryptedToken `toml:"tokens"`
}

type encryptedToken struct {
	Data string `toml:"data"` // base64(AES-256-GCM(toml-encoded Token))
}

// DefaultManager is the global token manager backed by ~/.bonsai.tokens.
var DefaultManager = &fileManager{}

type fileManager struct{}

func tokensPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".bonsai.tokens"), nil
}

func machineKey() []byte {
	// Derive a 32-byte key from the hostname. Not perfect security, but
	// keeps the file unreadable without context of the machine.
	hostname, _ := os.Hostname()
	sum := sha256.Sum256([]byte("bonsai-tokens-v1:" + hostname))
	return sum[:]
}

func encrypt(key, plaintext []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func decrypt(key []byte, encoded string) ([]byte, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(data) < gcm.NonceSize() {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := data[:gcm.NonceSize()], data[gcm.NonceSize():]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

func (m *fileManager) load() (tokenFile, error) {
	path, err := tokensPath()
	if err != nil {
		return tokenFile{}, err
	}
	var tf tokenFile
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return tokenFile{Tokens: map[string]encryptedToken{}}, nil
	}
	if _, err := toml.DecodeFile(path, &tf); err != nil {
		return tokenFile{}, err
	}
	if tf.Tokens == nil {
		tf.Tokens = map[string]encryptedToken{}
	}
	return tf, nil
}

func (m *fileManager) save(tf tokenFile) error {
	path, err := tokensPath()
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(tf)
}

// Get returns the token for the given host, or an error if none exists.
func (m *fileManager) Get(host string) (*Token, error) {
	tf, err := m.load()
	if err != nil {
		return nil, err
	}
	et, ok := tf.Tokens[host]
	if !ok {
		return nil, fmt.Errorf("no token for %s", host)
	}
	plain, err := decrypt(machineKey(), et.Data)
	if err != nil {
		return nil, fmt.Errorf("decrypt token for %s: %w", host, err)
	}
	var tok Token
	if _, err := toml.Decode(string(plain), &tok); err != nil {
		return nil, fmt.Errorf("parse token for %s: %w", host, err)
	}
	return &tok, nil
}

// Set stores (or overwrites) the token for its host.
func (m *fileManager) Set(tok Token) error {
	tf, err := m.load()
	if err != nil {
		return err
	}
	tok.SavedAt = time.Now()

	// TOML-encode the token for encryption.
	var buf []byte
	buf = append(buf, fmt.Sprintf("host = %q\n", tok.Host)...)
	buf = append(buf, fmt.Sprintf("access_token = %q\n", tok.AccessToken)...)
	buf = append(buf, fmt.Sprintf("token_type = %q\n", tok.TokenType)...)
	buf = append(buf, fmt.Sprintf("saved_at = %s\n", tok.SavedAt.Format(time.RFC3339))...)
	for _, s := range tok.Scopes {
		buf = append(buf, fmt.Sprintf("scopes = [%q]\n", s)...)
	}

	encrypted, err := encrypt(machineKey(), buf)
	if err != nil {
		return err
	}
	tf.Tokens[tok.Host] = encryptedToken{Data: encrypted}
	return m.save(tf)
}

// Delete removes the token for the given host.
func (m *fileManager) Delete(host string) error {
	tf, err := m.load()
	if err != nil {
		return err
	}
	delete(tf.Tokens, host)
	return m.save(tf)
}

// List returns all stored tokens (decrypted).
func (m *fileManager) List() ([]*Token, error) {
	tf, err := m.load()
	if err != nil {
		return nil, err
	}
	var tokens []*Token
	for host := range tf.Tokens {
		tok, err := m.Get(host)
		if err != nil {
			continue
		}
		tokens = append(tokens, tok)
	}
	return tokens, nil
}
