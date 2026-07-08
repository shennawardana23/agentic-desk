package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

// frontend/dist must live alongside this file — go:embed can only
// reach files at or below the source file's own directory, which is
// why frontend/ is co-located here rather than at the repo root
// PLAN.md originally sketched (see SESSION_HANDOFF.md's Phase 10
// note for the full reasoning).
//
//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:  "Agentic Desk",
		Width:  1100,
		Height: 780,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
		// Without Mac set at all, Wails' native bridge defaults `zoomable` to
		// false and explicitly disables the green traffic-light button
		// (window.go's zoomable-from-Mac.DisableZoom only runs when Mac != nil;
		// WailsContext.m then does `[button setEnabled:NO]` when !zoomable).
		Mac: &mac.Options{},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}
