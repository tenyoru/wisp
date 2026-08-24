package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"

	"wisp/internal/db"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	path, err := db.DefaultPath()
	if err != nil {
		log.Fatalf("wisp: resolve db path: %v", err)
	}
	store, err := db.Open(path)
	if err != nil {
		log.Fatalf("wisp: open db at %s: %v", path, err)
	}

	// application.Get() isn't valid until application.New() below returns,
	// so emit is a closure that resolves it lazily — by the time anything
	// actually calls emit, the app is running.
	feedSvc := &FeedService{
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
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "wisp",
		Width:  1000,
		Height: 618,
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
