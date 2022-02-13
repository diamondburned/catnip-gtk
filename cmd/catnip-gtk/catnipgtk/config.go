package catnipgtk

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/diamondburned/catnip-gtk"
	"github.com/diamondburned/gotk4-handy/pkg/handy"
	"github.com/diamondburned/gotk4/pkg/cairo"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/pkg/errors"
)

// UserConfigPath is the default path to the user's config file.
var UserConfigPath = filepath.Join(glib.GetUserConfigDir(), "catnip-gtk", "config.json")

type Config struct {
	Input      Input
	Appearance Appearance
	Visualizer Visualizer
	WindowSize struct {
		Width  int
		Height int
	}
}

// NewConfig creates a new default config.
func NewConfig() (*Config, error) {
	cfg := Config{
		Appearance: NewAppearance(),
		Visualizer: NewVisualizer(),
	}
	cfg.WindowSize.Width = 1000
	cfg.WindowSize.Height = 150

	input, err := NewInput()
	if err != nil {
		return nil, err
	}

	cfg.Input = input

	return &cfg, nil
}

// ReadUserConfig reads the user's config file at the default user path.
func ReadUserConfig() (*Config, error) {
	return ReadConfig(UserConfigPath)
}

// ReadConfig reads the config at the given path.
func ReadConfig(path string) (*Config, error) {
	c, err := NewConfig()
	if err != nil {
		return c, errors.Wrap(err, "failed to make default config")
	}

	f, err := os.Open(path)
	if err != nil {
		return c, errors.Wrap(err, "failed to open config path")
	}
	defer f.Close()

	if err := json.NewDecoder(f).Decode(c); err != nil {
		return c, errors.Wrap(err, "failed to decode JSON")
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

	window := handy.NewPreferencesWindow()
	window.SetSearchEnabled(true)
	window.SetModal(false)
	window.Add(input)
	window.Add(appearance)
	window.Add(visualizer)

	return window
}

// Transform turns this config into a catnip config.
func (cfg Config) Transform() catnip.Config {
	catnipCfg := catnip.Config{
		Backend:      cfg.Input.Backend,
		Device:       cfg.Input.Device,
		Monophonic:   !cfg.Input.DualChannel,
		WindowFn:     cfg.Visualizer.WindowFn.AsFunction(),
		SampleRate:   cfg.Visualizer.SampleRate,
		SampleSize:   cfg.Visualizer.SampleSize,
		SmoothFactor: cfg.Visualizer.SmoothFactor,
		MinimumClamp: cfg.Appearance.MinimumClamp,
		DrawStyle:    cfg.Appearance.DrawStyle,
		DrawOptions: catnip.DrawOptions{
			LineCap:    cfg.Appearance.LineCap.AsLineCap(),
			LineJoin:   cairo.LINE_JOIN_MITER,
			FrameRate:  cfg.Visualizer.FrameRate,
			BarWidth:   cfg.Appearance.BarWidth,
			SpaceWidth: cfg.Appearance.SpaceWidth,
			AntiAlias:  cfg.Appearance.AntiAlias.AsAntialias(),
			ForceEven:  false,
		},
		Scaling: catnip.ScalingConfig{
			SlowWindow:     5,
			FastWindow:     4,
			DumpPercent:    0.75,
			ResetDeviation: 1.0,
		},
	}

	if cfg.Appearance.ForegroundColor != nil {
		catnipCfg.DrawOptions.Colors.Foreground = cfg.Appearance.ForegroundColor
	}
	if cfg.Appearance.BackgroundColor != nil {
		catnipCfg.DrawOptions.Colors.Background = cfg.Appearance.BackgroundColor
	}

	return catnipCfg
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
