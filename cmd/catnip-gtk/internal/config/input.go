package config

import (
	"errors"

	"github.com/diamondburned/handy"
	"github.com/gotk3/gotk3/gtk"
	"github.com/noriah/catnip/input"
)

type Input struct {
	Backend     string
	Device      string
	DualChannel bool // .Monophonic

	backends []input.NamedBackend
	devices  map[string][]input.Device // first is always default
}

func NewInput() (Input, error) {
	if len(input.Backends) == 0 {
		return Input{}, errors.New("no input backends found")
	}

	ic := Input{
		backends: make([]input.NamedBackend, 0, len(input.Backends)),
		devices:  make(map[string][]input.Device, len(input.Backends)),
	}

	ic.Update()
	ic.Backend = ic.backends[0].Name
	ic.Device = ic.devices[ic.Backend][0].String()
	ic.DualChannel = true

	return ic, nil
}

// Update updates the list of input devices.
func (ic *Input) Update() {
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
	deviceComboCallback := deviceCombo.Connect("changed", func(deviceCombo *gtk.ComboBoxText) {
		if ix := deviceCombo.GetActive(); ix > 0 {
			ic.Device = ic.devices[ic.Backend][ix].String()
		} else {
			ic.Device = "" // default
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
	backendCombo.Connect("changed", func(backendCombo *gtk.ComboBoxText) {
		ic.Backend = backendCombo.GetActiveText()
		ic.Device = ""

		deviceCombo.HandlerBlock(deviceComboCallback)
		defer deviceCombo.HandlerUnblock(deviceComboCallback)

		// Update the list of devices when we're changing backend.
		ic.Update()

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

	dualCh, _ := gtk.SwitchNew()
	dualCh.SetVAlign(gtk.ALIGN_CENTER)
	dualCh.SetActive(ic.DualChannel)
	dualCh.Show()
	dualCh.Connect("state-set", func(dualCh *gtk.Switch, state bool) {
		ic.DualChannel = state
		apply()
	})

	dualChRow := handy.ActionRowNew()
	dualChRow.Add(dualCh)
	dualChRow.SetActivatableWidget(dualCh)
	dualChRow.SetTitle("Dual Channels")
	dualChRow.SetSubtitle("If enabled, will draw two channels mirrored instead of one.")
	dualChRow.Show()

	group := handy.PreferencesGroupNew()
	group.SetTitle("Input")
	group.Add(backendRow)
	group.Add(deviceRow)
	group.Add(dualChRow)
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
	return findDevice(ic.devices[ic.Backend], ic.Device)
}
