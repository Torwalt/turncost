{
  description = "turncost";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    # NOTE: nixos-unstable ships OpenCode 2; this pin is the last revision with OpenCode 1.
    nixpkgs-opencode.url = "github:NixOS/nixpkgs/4bcd09ff0157f78d3930877872a2405ee3ee1e9c";
    git-hooks = {
      url = "github:cachix/git-hooks.nix";
      inputs.nixpkgs.follows = "nixpkgs";
    };
    claude-code-nix.url = "github:sadjow/claude-code-nix";
    codex-cli-nix.url = "github:sadjow/codex-cli-nix";
    ploopy = {
      url = "github:Torwalt/ploopy";
      inputs.nixpkgs.follows = "nixpkgs";
      inputs.claude-code-nix.follows = "claude-code-nix";
    };
    benchpress = {
      url = "github:Torwalt/benchpress";
      inputs.nixpkgs.follows = "nixpkgs";
      inputs.claude-code-nix.follows = "claude-code-nix";
      inputs.ploopy.follows = "ploopy";
    };
  };

  outputs =
    {
      self,
      nixpkgs,
      nixpkgs-opencode,
      git-hooks,
      claude-code-nix,
      codex-cli-nix,
      ploopy,
      benchpress,
    }:
    let
      forAllSystems = nixpkgs.lib.genAttrs [
        "x86_64-linux"
        "aarch64-linux"
      ];
    in
    {
      packages = forAllSystems (system: {
        opencode = nixpkgs.legacyPackages.${system}.callPackage ./nix/opencode.nix {
          inherit (nixpkgs-opencode.legacyPackages.${system}) opencode;
        };
      });

      devShells = forAllSystems (
        system:
        let
          pkgs = nixpkgs.legacyPackages.${system};

          codex = codex-cli-nix.packages.${system}.default.overrideAttrs (old: {
            nativeBuildInputs = (old.nativeBuildInputs or [ ]) ++ [
              pkgs.patchelf
              pkgs.makeWrapper
            ];
            postInstall = (old.postInstall or "") + ''
              voice=$out/lib/codex/codex-resources/voice
              patchelf --set-interpreter ${pkgs.stdenv.cc.bintools.dynamicLinker} \
                --add-rpath ${pkgs.lib.getLib pkgs.glibc}/lib \
                "$voice/bin/codex-voice-host"
              wrapProgram "$voice/bin/codex-voice-host" \
                --set ALSA_CONFIG_PATH ${pkgs.writeText "codex-asound.conf" ''
                  <${pkgs.alsa-lib}/share/alsa/alsa.conf>
                  !@hooks []
                  defaults.namehint.showall on
                  defaults.namehint.basic on
                  defaults.namehint.extended on
                  pcm_type.pipewire { lib "${pkgs.pipewire}/lib/alsa-lib/libasound_module_pcm_pipewire.so" }
                  ctl_type.pipewire { lib "${pkgs.pipewire}/lib/alsa-lib/libasound_module_ctl_pipewire.so" }
                  pcm.!default {
                    type pipewire
                    hint { show on description "PipeWire" }
                  }
                  ctl.!default { type pipewire }
                ''}
            '';
          });

          hooks = git-hooks.lib.${system}.run {
            src = ./.;
            hooks.commit-message = {
              enable = true;
              name = "Reject AI attribution trailers and long commit bodies.";
              entry = "python3 tools/commit_msg_check.py";
              language = "system";
              pass_filenames = true;
              stages = [ "commit-msg" ];
            };
          };
        in
        {
          default = pkgs.mkShell {
            packages =
              (with pkgs; [
                just
                git
                gh
                tmux
                ripgrep
                fzf
                jq
                nixfmt
                files-to-prompt
                python3
                go
                gopls
                gotools
              ])
              ++ [
                self.packages.${system}.opencode
                claude-code-nix.packages.${system}.claude-code
                codex
                ploopy.packages.${system}.ploopy
                benchpress.packages.${system}.benchpress
              ]
              ++ hooks.enabledPackages;

            # NOTE: OpenCode 2 elsewhere on the machine creates opencode.db, which OpenCode 1 cannot open.
            OPENCODE_DB = "opencode-stable.db";

            shellHook = ''
              ${hooks.shellHook}
            '';
          };
        }
      );

      formatter = forAllSystems (system: nixpkgs.legacyPackages.${system}.nixfmt);
    };
}
