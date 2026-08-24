{ pkgs, ... }:

{
  languages.go.enable = true;
  languages.typescript.enable = true; # tsc + LSP only, not Node — see languages.javascript below

  languages.javascript = {
    enable = true;
    directory = "cmd/gui/frontend";
    npm.enable = true;
    npm.install.enable = true;
  };

  packages = with pkgs; [
    gopls
    pkg-config
    gcc
    gtk4
    webkitgtk_6_0
    ffmpeg
    whisper-cpp
    alsa-lib
    air
  ];

  # `go install .../wails3@latest` puts the CLI in $GOPATH/bin — put it on
  # PATH so `wails3` is callable directly inside the shell.
  enterShell = ''
    export PATH="$(go env GOPATH)/bin:$PATH"
  '';

  processes = {
    # Native-window dev loop: Vite HMR + auto Go rebuild, via `devenv up gui-dev`.
    # Same command the Wails-generated Taskfile's `dev` task runs.
    gui-dev.exec = "cd cmd/gui && wails3 dev -config ./build/config.yml -port 9245";

    # Browser-testable dev loop: `devenv up gui-server-watch frontend-watch`.
    # air rebuilds+restarts the -tags server binary on any .go change (see
    # .air.toml for why it also watches frontend/dist). Fixed port, distinct
    # from `just gui-web`'s (34115), so running both at once never collides.
    gui-server-watch = {
      exec = "air -c .air.toml";
      env.WAILS_SERVER_PORT = "34117";
    };
    # Continuously rebuilds frontend/dist on any frontend source change —
    # air picks that up and rebuilds the Go binary with the fresh embed.
    frontend-watch.exec = "cd cmd/gui/frontend && npm run build:dev -- --watch";
  };
}
