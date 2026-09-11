package main

import (
	"embed"
	"log"
	"net/http"

	"github.com/wailsapp/wails/v3/pkg/application"

	"wisp/internal/db"
	"wisp/internal/paths"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	if err := paths.Resolve(); err != nil {
		log.Fatalf("wisp: resolve app paths: %v", err)
	}
	store, err := db.Open(paths.DB)
	if err != nil {
		log.Fatalf("wisp: open db at %s: %v", paths.DB, err)
	}

	play := withCORS(&playServer{store: store})
	assetMux := http.NewServeMux()
	assetMux.Handle("/play/", play)
	assetMux.Handle("/", application.AssetFileServerFS(assets))

	go func() {
		mux := http.NewServeMux()
		mux.Handle("/play/", play)
		if err := http.ListenAndServe(episodeListen, mux); err != nil {
			log.Printf("wisp: episode server: %v", err)
		}
	}()

	feedSvc := &FeedService{ // emit is lazy: application.Get() is invalid until New() returns
		store: store,
		emit: func(event string, data ...any) {
			application.Get().Event.Emit(event, data...)
		},
	}

	app := application.New(application.Options{
		Name:        "wisp",
		Description: "RSS reader for podcasts and articles with local-Whisper shadowing practice",
		Services: []application.Service{
			application.NewService(feedSvc),
		},
		Assets: application.AssetOptions{
			Handler: assetMux,
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "wisp",
		Width:     1000,
		Height:    618,
		MinWidth:  640,
		MinHeight: 480,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 50,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(18, 17, 16),
		URL:              "/",
	})

	runErr := app.Run()
	store.Close()
	if runErr != nil {
		log.Fatal(runErr)
	}
}
