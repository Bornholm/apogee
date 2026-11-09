package client

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// Settings are the player's preferences, kept between sessions in the user's
// configuration directory. No personal data: a name the player chose.
type Settings struct {
	Name         string `json:"name"`
	TutorialDone bool   `json:"tutorialDone"`
}

// Dir returns the directory where Apogée keeps its files.
func Dir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "apogee"), nil
}

func settingsPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "settings.json"), nil
}

// LoadSettings reads the settings, or returns the defaults.
func LoadSettings() Settings {
	s := Settings{Name: "Dieu"}
	path, err := settingsPath()
	if err != nil {
		return s
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return s
	}
	if json.Unmarshal(data, &s) != nil || s.Name == "" {
		s.Name = "Dieu"
	}
	return s
}

// Save writes the settings.
func (s Settings) Save() error {
	path, err := settingsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// SoloSave returns the file of the solo sky, and whether it exists.
func SoloSave() (string, bool) {
	dir, err := Dir()
	if err != nil {
		return "", false
	}
	path := filepath.Join(dir, "solo.json")
	_, err = os.Stat(path)
	return path, !errors.Is(err, os.ErrNotExist) && err == nil
}
