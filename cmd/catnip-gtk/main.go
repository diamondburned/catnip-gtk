package main

import (
	"log"
	"os"

	"github.com/diamondburned/catnip-gtk/cmd/catnip-gtk/catnipgtk"
	"github.com/diamondburned/handy"
	"github.com/gotk3/gotk3/gdk"
	"github.com/gotk3/gotk3/gtk"

	// Required.
	_ "github.com/noriah/catnip/input/ffmpeg"
	_ "github.com/noriah/catnip/input/parec"
	_ "github.com/noriah/catnip/input/portaudio"
)

func init() {
	gtk.Init(&os.Args)
	handy.Init()
}

func main() {
	cfg, err := catnipgtk.ReadUserConfig()
	if err != nil {
		log.Fatalln("failed to read config:", err)
	}

	session := catnipgtk.NewSession(cfg)
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

func save(cfg *catnipgtk.Config) {
	if catnipgtk.UserConfigPath == "" {
		return
	}

	if err := cfg.Save(catnipgtk.UserConfigPath); err != nil {
		log.Println("failed to save config:", err)
	}
}
