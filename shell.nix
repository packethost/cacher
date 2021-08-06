let _pkgs = import <nixpkgs> { };
in { pkgs ? import (_pkgs.fetchFromGitHub {
  owner = "NixOS";
  repo = "nixpkgs";
  #branch@date: nixpkgs-unstable@2021-01-25
  rev = "869e4a894e7436441bdcb084846c60ae07a1dddd";
  sha256 = "1wdypsfjw4dmqkafa4px59pnzzhxzf8pwcsjff7f63z61wxgh5gh";
}) { } }:
with pkgs;

mkShell {
  buildInputs = [ cfssl dep go goimports go-protobuf golangci-lint protobuf ];
}
