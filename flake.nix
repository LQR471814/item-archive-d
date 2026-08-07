{
  inputs = {
    # self.submodules = true;
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };
  outputs =
    { self, nixpkgs }:
    let
      system = "x86_64-linux";
      pkgs = import nixpkgs { inherit system; };
    in
    {
      packages.${system}.default = pkgs.buildGoModule (_: {
        pname = "item-archive-d";
        version = "0.1.0";

        src = ./.;
        vendorHash = "sha256-ir5a7cJEmIL2XorNDv+Z2zY2UInMQv1KhndDvvuV0oE=";

        meta = {
          description = "A web application for keeping track of everything you've archived (or shoved something somewhere).";
          homepage = "https://github.com/LQR471814/item-archive-d";
          license = pkgs.lib.licenses.mit;
        };

        checkFlags = [
          "-skip=^TestDB$"
        ];
      });

      apps.${system}.default = {
        type = "app";
        program = "${self.packages.${system}.default}/bin/item-archive-d";
      };
    };
}
