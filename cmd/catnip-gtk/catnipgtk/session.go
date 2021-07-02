package catnipgtk

import (
	"fmt"
	"html"
	"log"

	"github.com/diamondburned/catnip-gtk"
	"github.com/gotk3/gotk3/glib"
	"github.com/gotk3/gotk3/gtk"
)

type Session struct {
	gtk.Stack

	Error *gtk.Label

	Area   *gtk.DrawingArea
	Drawer *catnip.Drawer

	css      *gtk.CssProvider
	config   *Config
	handlers []glib.SignalHandle
}

func NewSession(cfg *Config) *Session {
	errLabel, _ := gtk.LabelNew("")
	errLabel.Show()

	area, _ := gtk.DrawingAreaNew()
	area.Show()

	stack, _ := gtk.StackNew()
	stack.AddNamed(area, "area")
	stack.AddNamed(errLabel, "error")
	stack.SetVisibleChildName("area")
	stack.Show()

	css, _ := gtk.CssProviderNew()

	session := &Session{
		Stack: *stack,
		Error: errLabel,
		Area:  area,

		config: cfg,
		css:    css,
	}

	stack.Connect("realize", func(stack *gtk.Stack) {
		screen, _ := stack.GetScreen()
		gtk.AddProviderForScreen(
			screen, css,
			uint(gtk.STYLE_PROVIDER_PRIORITY_USER),
		)
	})

	return session
}

func (s Session) Stop() {
	for _, h := range s.handlers {
		s.Area.HandlerDisconnect(h)
	}
	s.handlers = nil

	if s.Drawer != nil {
		s.Drawer.Stop()
		s.Drawer = nil
	}
}

func (s *Session) verifyHandlers(handlers ...glib.SignalHandle) bool {
	if len(handlers) != len(s.handlers) {
		return false
	}

	for i, h := range handlers {
		if h != s.handlers[i] {
			return false
		}
	}

	return true
}

func (s *Session) Reload() {
	catnipCfg := s.config.Transform()

	if err := s.css.LoadFromData(s.config.Appearance.CustomCSS); err != nil {
		log.Println("CSS error:", err)
	}

	s.Stop()

	drawer := catnip.NewDrawer(s.Area, catnipCfg)
	drawer.SetWidgetStyle(s.Area)
	drawer.SetBackend(s.config.Input.InputBackend())
	drawer.SetDevice(s.config.Input.InputDevice())

	drawID := drawer.ConnectDraw(s.Area)
	stopID := drawer.ConnectDestroy(s.Area)

	s.Stack.SetVisibleChild(s.Area)
	s.Drawer = drawer
	s.handlers = []glib.SignalHandle{drawID, stopID}

	go func() {
		if err := drawer.Start(); err != nil {
			log.Println("Error starting Drawer:", err)
			glib.IdleAdd(func() {
				// Ensure this drawer is still being displayed.
				if s.verifyHandlers(drawID, stopID) {
					s.Error.SetMarkup(errorText(err))
					s.Stack.SetVisibleChild(s.Error)
				}
			})
		}
	}()
}

func errorText(err error) string {
	return fmt.Sprintf(
		`<span color="red"><b>Error:</b> %s</span>`,
		html.EscapeString(err.Error()),
	)
}
