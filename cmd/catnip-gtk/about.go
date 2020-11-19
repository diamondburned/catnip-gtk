package main

import (
	"strings"

	"github.com/gotk3/gotk3/gtk"
)

var Version = "tip"

var license = strings.TrimSpace(`
Permission to use, copy, modify, and/or distribute this software for any purpose
with or without fee is hereby granted, provided that the above copyright notice
and this permission notice appear in all copies.

THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES WITH
REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF MERCHANTABILITY AND
FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR ANY SPECIAL, DIRECT,
INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES WHATSOEVER RESULTING FROM LOSS
OF USE, DATA OR PROFITS, WHETHER IN AN ACTION OF CONTRACT, NEGLIGENCE OR OTHER
TORTIOUS ACTION, ARISING OUT OF OR IN CONNECTION WITH THE USE OR PERFORMANCE OF
THIS SOFTWARE.
`)

func About() *gtk.AboutDialog {
	about, _ := gtk.AboutDialogNew()
	about.SetModal(true)
	about.SetProgramName("catnip-gtk")
	about.SetVersion(Version)
	about.SetLicenseType(gtk.LICENSE_MIT_X11)
	about.SetLicense(license)
	about.SetAuthors([]string{
		"diamondburned",
		"noriah reuland (code@noriah.dev)",
	})

	return about
}
