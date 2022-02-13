package main

import (
	"log"
	"os"

	"github.com/diamondburned/catnip-gtk/cmd/catnip-gtk/catnipgtk"
	"github.com/diamondburned/gotk4-handy/pkg/handy"
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gdk/v3"
	"github.com/diamondburned/gotk4/pkg/gtk/v3"

	// Required.
	"github.com/noriah/catnip/dsp"
	_ "github.com/noriah/catnip/input/ffmpeg"
	_ "github.com/noriah/catnip/input/parec"
	_ "github.com/noriah/catnip/input/portaudio"
)

func init() {
	// Weird hacks to reduce catnip's frequency scaling:

	// Start drawing at 20Hz, not 60Hz.
	dsp.Frequencies[1] = 20

	// Don't scale frequencies below 250Hz differently.
	dsp.Frequencies[2] = 0
}

func main() {
	cfg, err := catnipgtk.ReadUserConfig()
	if err != nil {
		log.Fatalln("failed to read config:", err)
	}

	app := gtk.NewApplication("com.github.diamondburned.catnip-gtk", 0)
	app.ConnectActivate(func() {
		handy.Init()

		session := catnipgtk.NewSession(cfg)
		session.Reload()
		session.Show()

		evbox := gtk.NewEventBox()
		evbox.Add(session)
		evbox.Show()

		h := handy.NewWindowHandle()
		h.Add(evbox)
		h.Show()

		w := handy.NewApplicationWindow()
		w.SetApplication(app)
		w.SetDefaultSize(cfg.WindowSize.Width, cfg.WindowSize.Height)
		w.Add(h)
		w.Show()

		var resizeSave glib.SourceHandle
		w.Connect("size-allocate", func() {
			cfg.WindowSize.Width = w.AllocatedWidth()
			cfg.WindowSize.Height = w.AllocatedHeight()

			if resizeSave == 0 {
				resizeSave = glib.TimeoutSecondsAdd(1, func() {
					save(cfg)
					resizeSave = 0
				})
			}
		})

		wstyle := w.StyleContext()
		wstyle.AddClass("catnip")

		prefMenu := gtk.NewMenuItemWithLabel("Preferences")
		prefMenu.Show()
		prefMenu.Connect("activate", func(prefMenu *gtk.MenuItem) {
			cfgw := cfg.PreferencesWindow(session.Reload)
			cfgw.Connect("destroy", func(*handy.PreferencesWindow) { save(cfg) })
			cfgw.Show()
		})

		aboutMenu := gtk.NewMenuItemWithLabel("About")
		aboutMenu.Show()
		aboutMenu.Connect("activate", func(aboutMenu *gtk.MenuItem) {
			about := About()
			about.SetTransientFor(&w.Window)
			about.Show()
		})

		quitMenu := gtk.NewMenuItemWithLabel("Quit")
		quitMenu.Show()
		quitMenu.Connect("activate", func(*gtk.MenuItem) { w.Destroy() })

		menu := gtk.NewMenu()
		menu.Append(prefMenu)
		menu.Append(aboutMenu)
		menu.Append(quitMenu)

		evbox.Connect("button-press-event", func(evbox *gtk.EventBox, ev *gdk.Event) {
			if b := ev.AsButton(); b.Button() == gdk.BUTTON_SECONDARY {
				menu.PopupAtPointer(ev)
			}
		})
	})

	os.Exit(app.Run(os.Args))
}

func save(cfg *catnipgtk.Config) {
	if catnipgtk.UserConfigPath == "" {
		return
	}

	if err := cfg.Save(catnipgtk.UserConfigPath); err != nil {
		log.Println("failed to save config:", err)
	}
}
