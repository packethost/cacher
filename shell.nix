with import <nixpkgs> { };
stdenv.mkDerivation rec {
  name = "cacher";
  env = buildEnv { name = name; paths = buildInputs; };
  buildInputs = [
    cfssl
    go
    protobuf
  ];
}
