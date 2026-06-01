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

            vendorHash = "sha256-sB4BU+dewyCncBj3eoyKcnjGqB4Jk8/eDY0+4dmZsUk=";

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

            postInstall = ''
              install -Dm644 <($out/bin/composia completion bash) \
                $out/share/bash-completion/completions/composia
              install -Dm644 <($out/bin/composia completion zsh) \
                $out/share/zsh/site-functions/_composia
              install -Dm644 <($out/bin/composia completion fish) \
                $out/share/fish/vendor_completions.d/composia.fish
            '';

            meta = {
              description = "Self-hosted Docker Compose control plane and CLI";
              homepage = "https://composia.xyz";
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
