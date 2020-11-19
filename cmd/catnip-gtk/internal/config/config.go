package config

import (
	"encoding/json"
	"os"

	"github.com/diamondburned/handy"
	"github.com/pkg/errors"
)

type Config struct {
	Input      Input
	Appearance Appearance
	Visualizer Visualizer
}

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
