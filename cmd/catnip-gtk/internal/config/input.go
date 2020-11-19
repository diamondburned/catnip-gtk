package config

import (
	"errors"

	"github.com/diamondburned/handy"
	"github.com/gotk3/gotk3/gtk"
	"github.com/noriah/catnip/input"
)

type Input struct {
	Backend string
	Device  string

	backends []input.NamedBackend
	devices  map[string][]input.Device // first is always default
	current  int
}

func NewInput() (Input, error) {
	if len(input.Backends) == 0 {
		return Input{}, errors.New("no input backends found")
	}

	ic := Input{
		backends: make([]input.NamedBackend, 0, len(input.Backends)),
		devices:  make(map[string][]input.Device, len(input.Backends)),
	}

	for _, backend := range input.Backends {
		devices, _ := backend.Devices()
		defdevc, _ := backend.DefaultDevice()

		// Skip broken backends.
		if len(devices) == 0 && defdevc == nil {
			continue
		}

		ic.backends = append(ic.backends, backend)

		// Fallback to the first device if there is no default.
		if defdevc == nil {
			defdevc = devices[0]
		}

		ic.devices[backend.Name] = append([]input.Device{defdevc}, devices...)
	}

	ic.Backend = ic.backends[0].Name
	ic.Device = ic.devices[ic.Backend][0].String()

	return ic, nil
}

func (ic *Input) Page(apply func()) *handy.PreferencesPage {
	deviceCombo, _ := gtk.ComboBoxTextNew()
	deviceCombo.SetVAlign(gtk.ALIGN_CENTER)
	deviceCombo.Show()

	backendCombo, _ := gtk.ComboBoxTextNew()
	backendCombo.SetVAlign(gtk.ALIGN_CENTER)
	backendCombo.Show()

	addDeviceCombo(deviceCombo, ic.devices[ic.Backend])
	if device := findDevice(ic.devices[ic.Backend], ic.Device); device != nil {
		deviceCombo.SetActiveID("__" + ic.Device)
	} else {
		deviceCombo.SetActive(0)
		ic.Device = ""
	}
	deviceComboCallback, _ := deviceCombo.Connect("changed", func() {
		ic.current = deviceCombo.GetActive()
		if ic.current > 0 {
			ic.Device = ic.devices[ic.Backend][ic.current].String()
		} else {
			ic.Device = "" // default
			ic.current = 0
		}

		apply()
	})

	addBackendCombo(backendCombo, ic.backends)
	if backend := findBackend(ic.backends, ic.Backend); backend.Backend != nil {
		backendCombo.SetActiveID(ic.Backend)
	} else {
		backendCombo.SetActive(0)
		ic.Backend = input.Backends[0].Name
	}
	backendCombo.Connect("changed", func() {
		ic.Backend = backendCombo.GetActiveText()
		ic.Device = ""

		deviceCombo.HandlerBlock(deviceComboCallback)
		defer deviceCombo.HandlerUnblock(deviceComboCallback)

		deviceCombo.RemoveAll()
		addDeviceCombo(deviceCombo, ic.devices[ic.Backend])
		deviceCombo.SetActive(0)

		apply()
	})

	backendRow := handy.ActionRowNew()
	backendRow.SetTitle("Backend")
	backendRow.SetSubtitle("The backend to use for audio input.")
	backendRow.Add(backendCombo)
	backendRow.SetActivatableWidget(backendCombo)
	backendRow.Show()

	deviceRow := handy.ActionRowNew()
	deviceRow.SetTitle("Device")
	deviceRow.SetSubtitle("The device to use for audio input.")
	deviceRow.Add(deviceCombo)
	deviceRow.SetActivatableWidget(deviceCombo)
	deviceRow.Show()

	group := handy.PreferencesGroupNew()
	group.SetTitle("Input")
	group.Add(backendRow)
	group.Add(deviceRow)
	group.Show()

	page := handy.PreferencesPageNew()
	page.SetTitle("Audio")
	page.SetIconName("audio-card-symbolic")
	page.Add(group)

	return page
}

func addDeviceCombo(deviceCombo *gtk.ComboBoxText, devices []input.Device) {
	for i, device := range devices {
		if i == 0 {
			deviceCombo.Append("default", "Default")
		} else {
			name := device.String()
			deviceCombo.Append("__"+name, name)
		}
	}
}

func addBackendCombo(backendCombo *gtk.ComboBoxText, backends []input.NamedBackend) {
	for _, backend := range backends {
		backendCombo.Append(backend.Name, backend.Name)
	}
}

func findDevice(devices []input.Device, str string) input.Device {
	if str == "" {
		return nil
	}
	for _, device := range devices {
		if device.String() == str {
			return device
		}
	}
	return nil
}

func findBackend(backends []input.NamedBackend, str string) input.NamedBackend {
	if str == "" {
		return input.NamedBackend{}
	}
	for _, backend := range backends {
		if backend.Name == str {
			return backend
		}
	}
	return input.NamedBackend{}
}

func (ic *Input) InputBackend() input.Backend {
	return findBackend(ic.backends, ic.Backend).Backend
}

func (ic *Input) InputDevice() input.Device {
	return ic.devices[ic.Backend][ic.current]
}
