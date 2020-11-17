package main

import (
	"log"
	"os"

	"github.com/diamondburned/catnip-gtk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"

	// Required.
	_ "github.com/noriah/catnip/input/ffmpeg"
	_ "github.com/noriah/catnip/input/parec"
	_ "github.com/noriah/catnip/input/portaudio"
)

func main() {
	gtk.Init(&os.Args)

	config := catnip.NewConfig()
	config.SampleRate = 44100
	config.SampleSize = int(config.SampleRate / 70) // 70fps
	config.Backend = "parec"                        // use PulseAudio
	config.BarWidth = 4
	config.SpaceWidth = 1
	config.SmoothFactor = 39.29
	config.Monophonic = true
	config.ForceEven = true // sharpen

	a := catnip.New(config)
	a.Show()

	go func() {
		if err := a.Start(); err != nil {
			glib.IdleAdd(func() {
				gtk.MainQuit()
				log.Fatalln("failed to start catnip:", err)
			})
		}
	}()

	pause, _ := gtk.ToggleButtonNewWithLabel("Pause")
	pause.Connect("toggled", func() {
		a.SetPaused(pause.GetActive())
	})
	pause.SetHAlign(gtk.ALIGN_CENTER)
	pause.Show()

	box, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	box.PackStart(a, true, true, 0)
	box.PackStart(pause, false, false, 8)
	box.Show()

	w, _ := gtk.WindowNew(gtk.WINDOW_TOPLEVEL)
	w.SetDefaultSize(1000, 150)
	w.Add(box)
	w.Show()
	w.Connect("destroy", gtk.MainQuit)

	gtk.Main()
}
