package usage

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Data tracks per-command usage counts and user preferences about education panels.
type Data struct {
	Counts     map[string]int  `json:"counts"`
	Suppressed map[string]bool `json:"suppressed"` // user chose to stop seeing tips
	Prompted   map[string]bool `json:"prompted"`   // mastery question was already shown
}

// Load reads usage data from path. Returns empty Data if the file does not exist.
func Load(path string) (*Data, error) {
	d := &Data{
		Counts:     map[string]int{},
		Suppressed: map[string]bool{},
		Prompted:   map[string]bool{},
	}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return d, nil
	}
	if err != nil {
		return d, err
	}
	if err := json.Unmarshal(b, d); err != nil {
		return d, err
	}
	if d.Counts == nil {
		d.Counts = map[string]int{}
	}
	if d.Suppressed == nil {
		d.Suppressed = map[string]bool{}
	}
	if d.Prompted == nil {
		d.Prompted = map[string]bool{}
	}
	return d, nil
}

// Save writes usage data to path, creating parent directories as needed.
func (d *Data) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// Increment adds one use to the command key and returns the new count.
func (d *Data) Increment(key string) int {
	d.Counts[key]++
	return d.Counts[key]
}

// Count returns the number of times the command key has been used.
func (d *Data) Count(key string) int {
	return d.Counts[key]
}

// IsSuppressed reports whether the user chose to stop seeing tips for key.
func (d *Data) IsSuppressed(key string) bool {
	return d.Suppressed[key]
}

// Suppress marks a command key as suppressed.
func (d *Data) Suppress(key string) {
	d.Suppressed[key] = true
}

// WasPrompted reports whether the mastery question was already shown for key.
func (d *Data) WasPrompted(key string) bool {
	return d.Prompted[key]
}

// SetPrompted marks that the mastery question was shown for key.
func (d *Data) SetPrompted(key string) {
	d.Prompted[key] = true
}
