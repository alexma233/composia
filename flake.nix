{
  description = "Composia CLI and runtime binaries";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs = { self, nixpkgs }:
    let
      supportedSystems = [
        "x86_64-linux"
        "aarch64-linux"
        "i686-linux"
        "armv7l-linux"
        "riscv64-linux"
      ];

      forAllSystems = nixpkgs.lib.genAttrs supportedSystems;
    in
    {
      packages = forAllSystems (system:
        let
          pkgs = import nixpkgs { inherit system; };
          version =
            if self ? tag then self.tag
            else if self ? rev then "git-${self.shortRev}"
            else "dirty";
        in
        {
          composia = pkgs.buildGoModule {
            pname = "composia";
            inherit version;
            src = ./.;

            vendorHash = "sha256-LIts6L6jl2ZmHvOBboB9eBIf3VWraaVMca7/s7h65bU=";

            subPackages = [
              "cmd/composia"
              "cmd/composia-controller"
              "cmd/composia-agent"
            ];

            ldflags = [
              "-s"
              "-w"
              "-X forgejo.alexma.top/alexma233/composia/internal/version.Value=${version}"
            ];

            meta = {
              description = "Self-hosted Docker Compose control plane and CLI";
              homepage = "https://docs.composia.xyz";
              license = pkgs.lib.licenses.agpl3Only;
              mainProgram = "composia";
              platforms = supportedSystems;
            };
          };

          default = self.packages.${system}.composia;
        });

      apps = forAllSystems (system: {
        composia = {
          type = "app";
          program = "${self.packages.${system}.composia}/bin/composia";
        };
        composia-controller = {
          type = "app";
          program = "${self.packages.${system}.composia}/bin/composia-controller";
        };
        composia-agent = {
          type = "app";
          program = "${self.packages.${system}.composia}/bin/composia-agent";
        };
        default = self.apps.${system}.composia;
      });
    };
}
