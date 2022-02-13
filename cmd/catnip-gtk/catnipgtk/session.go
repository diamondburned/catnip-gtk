package catnipgtk

import (
	"fmt"
	"html"
	"log"

	"github.com/diamondburned/catnip-gtk"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v3"
)

type Session struct {
	gtk.Stack

	Error *gtk.Label

	Area   *gtk.DrawingArea
	Drawer *catnip.Drawer

	css    *gtk.CSSProvider
	config *Config
	saving glib.SourceHandle
}

func NewSession(cfg *Config) *Session {
	errLabel := gtk.NewLabel("")
	errLabel.Show()

	area := gtk.NewDrawingArea()
	area.Show()

	stack := gtk.NewStack()
	stack.AddNamed(area, "area")
	stack.AddNamed(errLabel, "error")
	stack.SetVisibleChildName("area")
	stack.Show()

	css := gtk.NewCSSProvider()

	session := &Session{
		Stack: *stack,
		Error: errLabel,
		Area:  area,

		config: cfg,
		css:    css,
	}

	stack.Connect("realize", func(stack *gtk.Stack) {
		gtk.StyleContextAddProviderForScreen(
			stack.Screen(), css,
			uint(gtk.STYLE_PROVIDER_PRIORITY_USER),
		)
	})

	return session
}

func (s Session) Stop() {
	if s.Drawer != nil {
		s.Drawer.Stop()
		s.Drawer = nil
	}
}

func (s *Session) Reload() {
	if s.saving == 0 {
		s.saving = glib.TimeoutAdd(150, func() {
			s.reload()
			s.saving = 0
		})
	}
}

func (s *Session) reload() {
	catnipCfg := s.config.Transform()

	if err := s.css.LoadFromData(s.config.Appearance.CustomCSS); err != nil {
		log.Println("CSS error:", err)
	}

	s.Stop()

	drawer := catnip.NewDrawer(s.Area, catnipCfg)
	drawer.SetBackend(s.config.Input.InputBackend())
	drawer.SetDevice(s.config.Input.InputDevice())

	s.Stack.SetVisibleChild(s.Area)
	s.Drawer = drawer

	go func() {
		if err := drawer.Start(); err != nil {
			log.Println("Error starting Drawer:", err)
			glib.IdleAdd(func() {
				// Ensure this drawer is still being displayed.
				if s.Drawer == drawer {
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
