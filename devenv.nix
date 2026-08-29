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
    glib-networking
    ffmpeg
    whisper-cpp
    alsa-lib
    air
  ] ++ (with gst_all_1; [
    gstreamer
    gst-plugins-base
    gst-plugins-good
    gst-plugins-bad
    gst-plugins-ugly
  ]);

  # `go install .../wails3@latest` puts the CLI in $GOPATH/bin — put it on
  # PATH so `wails3` is callable directly inside the shell.
  enterShell = ''
    export PATH="$(go env GOPATH)/bin:$PATH"
    export GIO_EXTRA_MODULES="${pkgs.glib-networking}/lib/gio/modules:$GIO_EXTRA_MODULES"
    export GST_PLUGIN_SYSTEM_PATH_1_0="${pkgs.lib.makeSearchPath "lib/gstreamer-1.0" (with pkgs.gst_all_1; [
      gstreamer.out
      gst-plugins-base.out
      gst-plugins-good.out
      gst-plugins-bad.out
      gst-plugins-ugly.out
    ])}"
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
