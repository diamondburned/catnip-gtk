package catnipgtk

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/diamondburned/handy"
	"github.com/gotk3/gotk3/glib"
	"github.com/pkg/errors"
)

// UserConfigPath is the default path to the user's config file.
var UserConfigPath = filepath.Join(glib.GetUserConfigDir(), "catnip-gtk", "config.json")

type Config struct {
	Input      Input
	Appearance Appearance
	Visualizer Visualizer
}

// NewConfig creates a new default config.
func NewConfig() (*Config, error) {
	cfg := Config{
		Appearance: NewAppearance(),
		Visualizer: NewVisualizer(),
	}

	input, err := NewInput()
	if err != nil {
		return nil, err
	}

	cfg.Input = input

	return &cfg, nil
}

// ReadUserConfig reads the user's config file at the default user path.
func ReadUserConfig() (*Config, error) {
	c, err := ReadConfig(UserConfigPath)
	if err == nil {
		return c, nil
	}
	return NewConfig()
}

// ReadConfig reads the config at the given path.
func ReadConfig(path string) (*Config, error) {
	c, err := NewConfig()
	if err != nil {
		return nil, errors.Wrap(err, "failed to make default config")
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open config path")
	}
	defer f.Close()

	if err := json.NewDecoder(f).Decode(c); err != nil {
		return nil, errors.Wrap(err, "failed to decode JSON")
	}

	return c, nil
}

func (cfg *Config) PreferencesWindow(apply func()) *handy.PreferencesWindow {
	// Refresh the input devices.
	cfg.Input.Update()

	input := cfg.Input.Page(apply)
	input.Show()

	appearance := cfg.Appearance.Page(apply)
	appearance.Show()

	visualizer := cfg.Visualizer.Page(apply)
	visualizer.Show()

	window := handy.PreferencesWindowNew()
	window.SetSearchEnabled(true)
	window.SetModal(true)
	window.Add(input)
	window.Add(appearance)
	window.Add(visualizer)

	return window
}

func (cfg Config) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), os.ModePerm); err != nil {
		return errors.Wrap(err, "failed to mkdir -p")
	}

	f, err := os.Create(path)
	if err != nil {
		return errors.Wrap(err, "failed to create config file")
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "\t")

	if err := enc.Encode(cfg); err != nil {
		return errors.Wrap(err, "failed to encode JSON")
	}

	return nil
}
