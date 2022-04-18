{ unstable ? import <nixpkgs> {} }:

let go = unstable.go.overrideAttrs (old: {
		version = "1.17.6";
		src = builtins.fetchurl {
			url    = "https://go.dev/dl/go1.17.6.src.tar.gz";
			sha256 = "sha256:1j288zwnws3p2iv7r938c89706hmi1nmwd8r5gzw3w31zzrvphad";
		};
		doCheck = false;
		patches = [
			# cmd/go/internal/work: concurrent ccompile routines
			(builtins.fetchurl "https://github.com/diamondburned/go/commit/4e07fa9fe4e905d89c725baed404ae43e03eb08e.patch")
			# cmd/cgo: concurrent file generation
			(builtins.fetchurl "https://github.com/diamondburned/go/commit/432db23601eeb941cf2ae3a539a62e6f7c11ed06.patch")
		];
	});

in unstable.stdenv.mkDerivation rec {
	name = "catnip-gtk";
	version = "0.0.1";

	CGO_ENABLED = "1";

	buildInputs = with unstable; [
		gtk3
		glib
		gtk-layer-shell
		gdk-pixbuf
		gobject-introspection
		libhandy
		gtk-layer-shell
		fftw
		portaudio
	];

	nativeBuildInputs = [ go unstable.pkgconfig ];
}
