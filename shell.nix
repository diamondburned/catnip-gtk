{ pkgs ? import <nixpkgs> {} }:

pkgs.stdenv.mkDerivation rec {
	name = "catnip-gtk";
	version = "0.0.1";

	buildInputs = with pkgs; [
		gnome3.glib gnome3.gtk libhandy fftw portaudio
	];

	nativeBuildInputs = with pkgs; [
		pkgconfig go
	];
}
