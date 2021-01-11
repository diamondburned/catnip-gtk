package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/diamondburned/catnip-gtk/cmd/catnip-gtk/internal/config"
	"github.com/diamondburned/handy"
	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"

	// Required.
	_ "github.com/noriah/catnip/input/ffmpeg"
	_ "github.com/noriah/catnip/input/parec"
	_ "github.com/noriah/catnip/input/portaudio"
)

var configPath = filepath.Join(glib.GetUserConfigDir(), "catnip-gtk", "config.json")

func init() {
	gtk.Init(&os.Args)
	handy.Init()

	if err := os.MkdirAll(filepath.Dir(configPath), os.ModePerm); err != nil {
		log.Println("failed to make config directory:", err)
		configPath = ""
	}
}

func main() {
	cfg, err := config.ReadConfig(configPath)
	if err != nil {
		log.Println("failed to load config, using default:", err)
		cfg, err = config.NewConfig()
	}
	if err != nil {
		log.Fatalln("failed to create config:", err)
	}

	session := NewSession(cfg)
	session.Reload()
	session.Show()

	evbox, _ := gtk.EventBoxNew()
	evbox.Add(session)
	evbox.Show()

	w := handy.WindowNew()
	w.Add(evbox)
	w.SetDefaultSize(1000, 150)
	w.Connect("destroy", func(w *handy.Window) { gtk.MainQuit() })
	w.Show()

	wstyle, _ := w.GetStyleContext()
	wstyle.AddClass("catnip")

	prefMenu, _ := gtk.MenuItemNewWithLabel("Preferences")
	prefMenu.Show()
	prefMenu.Connect("activate", func(prefMenu *gtk.MenuItem) {
		cfgw := cfg.PreferencesWindow(session.Reload)
		cfgw.Connect("destroy", func(*handy.PreferencesWindow) { save(cfg) })
		cfgw.Show()
	})

	aboutMenu, _ := gtk.MenuItemNewWithLabel("About")
	aboutMenu.Show()
	aboutMenu.Connect("activate", func(aboutMenu *gtk.MenuItem) {
		about := About()
		about.SetTransientFor(w)
		about.Show()
	})

	quitMenu, _ := gtk.MenuItemNewWithLabel("Quit")
	quitMenu.Show()
	quitMenu.Connect("activate", func(*gtk.MenuItem) { w.Destroy() })

	menu, _ := gtk.MenuNew()
	menu.Append(prefMenu)
	menu.Append(aboutMenu)
	menu.Append(quitMenu)

	evbox.Connect("button-press-event", func(evbox *gtk.EventBox, ev *gdk.Event) {
		if b := gdk.EventButtonNewFromEvent(ev); b.Button() == gdk.BUTTON_SECONDARY {
			menu.PopupAtPointer(ev)
		}
	})

	gtk.Main()
}

func save(cfg *config.Config) {
	if configPath == "" {
		return
	}

	if err := cfg.Save(configPath); err != nil {
		log.Println("failed to save config:", err)
	}
}
