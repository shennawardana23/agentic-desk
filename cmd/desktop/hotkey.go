package main

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.design/x/hotkey"
)

// windowVisible tracks whether we consider the window shown.
// Atomic so hotkeyLoop goroutine and ToggleWindow are race-free.
var windowVisible atomic.Bool

func init() { windowVisible.Store(true) } // window starts visible

// hotkeyRegistered is set to true once CGEventTap is active.
var hotkeyRegistered atomic.Bool

// HotkeyStatus returns whether ⌘⇧Space is registered and ready.
// Called by the frontend to show a hint when Accessibility is not granted.
func (a *App) HotkeyStatus() bool { return hotkeyRegistered.Load() }

// hotkeyLoop registers ⌘⇧Space as a global hotkey that toggles the main
// window. Retries every 2s if registration fails — handles the case
// where the user grants Accessibility permission while the app is running
// (AXIsProcessTrusted is re-checked on every attempt).
func (a *App) hotkeyLoop(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}

		hk := hotkey.New([]hotkey.Modifier{hotkey.ModCmd, hotkey.ModShift}, hotkey.KeySpace)
		if err := hk.Register(); err != nil {
			slog.Warn("voice hotkey: ⌘⇧Space not registered — Accessibility permission required", "retry_in", "2s")
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
			continue
		}

		slog.Info("voice hotkey: ⌘⇧Space registered — press from any app to show/hide")
		hotkeyRegistered.Store(true)
		for {
			select {
			case <-ctx.Done():
				hk.Unregister()
				return
			case <-hk.Keydown():
				a.ToggleWindow()
			}
		}
	}
}

// ToggleWindow shows the window if hidden, hides it if visible.
// Uses our own atomic flag — more reliable than WindowIsNormal, which
// also returns false for minimised windows.
func (a *App) ToggleWindow() {
	if windowVisible.Swap(false) {
		runtime.WindowHide(a.ctx)
	} else {
		windowVisible.Store(true)
		runtime.WindowShow(a.ctx)
	}
}
