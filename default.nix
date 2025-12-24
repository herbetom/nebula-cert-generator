{ pkgs ? import <nixpkgs> {} }:

pkgs.buildGoModule {
  pname = "nebula-cert-generator";
  version = "0-unstable-2025-12-23";

  src = ./.;
  vendorHash = "sha256-g+yaVIx4jxpAQ/+WrGKxhVeliYx7nLQe/zsGpxV4Fn4=";

  # Optional: specify the main package
  # subPackages = [ "." ];

  # Optional build flags
  ldflags = [
    "-s"
    "-w"
  ];

  meta = with pkgs.lib; {
    description = "My Go application";
    license = licenses.eupl12;
  };
}
