package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	service := newFileService()
	app := application.New(application.Options{
		Name: "XFile", Description: "本地文件浏览与批量归并", Services: []application.Service{application.NewService(service)},
		Assets: application.AssetOptions{Handler: application.BundledAssetFileServer(assets)},
		Mac:    application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: true},
	})
	service.app = app
	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title: "XFile", Width: 1360, Height: 880, MinWidth: 1000, MinHeight: 680, URL: "/",
		Mac: application.MacWindow{
			TitleBar:   application.MacTitleBarHiddenInset,
			Backdrop:   application.MacBackdropLiquidGlass,
			Appearance: application.NSAppearanceNameAqua,
			LiquidGlass: application.MacLiquidGlass{
				Style:    application.LiquidGlassStyleAutomatic,
				Material: application.NSVisualEffectMaterialSidebar,
			},
		},
	})
	if e := app.Run(); e != nil {
		log.Fatal(e)
	}
}
