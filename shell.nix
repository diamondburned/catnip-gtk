{ unstable ? import <unstable> {} }:

unstable.stdenv.mkDerivation rec {
	name = "catnip-gtk";
	version = "0.0.1";

	buildInputs = with unstable; [
		gnome3.glib gnome3.gtk libhandy gtk-layer-shell fftw portaudio
	];

	nativeBuildInputs = with unstable; [
		pkgconfig go
	];
}
