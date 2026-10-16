package main

import (
	"embed"
	"fmt"
	"log"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	khali, err := NewKhali()
	if err != nil {
		log.Fatal(err)
	}

	app := application.New(application.Options{
		Name:        "Khali",
		Description: "Frees disk space taken by node_modules, build output and developer caches",
		Services: []application.Service{
			application.NewService(khali),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			// Closing the window keeps khali in the menu bar.
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
	})

	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "Khali",
		Width:     1100,
		Height:    740,
		MinWidth:  760,
		MinHeight: 520,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 52,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundColour: application.NewRGB(250, 250, 250),
		URL:              "/",
	})
	window.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		window.Hide()
		e.Cancel()
	})

	show := func() {
		window.Show()
		window.Focus()
	}
	// Clicking the Dock icon brings the hidden window back.
	app.Event.OnApplicationEvent(events.Mac.ApplicationShouldHandleReopen, func(*application.ApplicationEvent) { show() })

	tray := app.SystemTray.New()
	menu := app.NewMenu()
	freeItem := menu.Add("").SetEnabled(false)
	menu.AddSeparator()
	menu.Add("Open Khali").OnClick(func(*application.Context) { show() })
	menu.AddSeparator()
	menu.Add("Quit Khali").OnClick(func(*application.Context) { app.Quit() })
	tray.SetMenu(menu)

	refresh := func() {
		d := khali.Disk()
		tray.SetLabel(gb(d.Free))
		freeItem.SetLabel(fmt.Sprintf("%s free of %s", gb(d.Free), gb(d.Total)))
		menu.Update()
	}
	khali.onChange = refresh
	refresh()
	go func() {
		for range time.Tick(time.Minute) {
			refresh()
		}
	}()

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

func gb(b int64) string {
	return fmt.Sprintf("%.1f GB", float64(b)/1e9)
}
