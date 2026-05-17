package overlay

import (
	"context"
	"live-captions/audio"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx          context.Context
	audioManager *audio.Manager

	// Saved window position before settings opens — restored on close.
	savedX, savedY int
}

func NewApp(am *audio.Manager) *App {
	return &App{audioManager: am}
}

func (a *App) OnStartup(ctx context.Context) {
	a.ctx = ctx

	// Wire the audio level callback — fires at ~20 Hz from the audio goroutine.
	// We scale RMS [0–1] to an integer 0–100 for a compact JSON payload.
	a.audioManager.LevelCallback = func(rms float32) {
		level := int(rms * 100)
		if level > 100 {
			level = 100
		}
		runtime.EventsEmit(a.ctx, "audio_level", level)
	}
}

// Called from main goroutine to push captions to frontend
func (a *App) EmitCaption(text string) {
	runtime.EventsEmit(a.ctx, "caption", text)
}

func (a *App) GetMicrophones() ([]map[string]interface{}, error) {
	return a.audioManager.GetMicrophones()
}

func (a *App) SwitchAudioSource(useMic bool, index int) error {
	return a.audioManager.SwitchSource(useMic, index)
}

// SelectFolder opens a native directory picker and returns the chosen path.
// Returns an empty string if the user cancels.
func (a *App) SelectFolder() (string, error) {
	// Temporarily release always-on-top so the dialog appears above the window
	runtime.WindowSetAlwaysOnTop(a.ctx, false)
	defer runtime.WindowSetAlwaysOnTop(a.ctx, true)

	path, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Transcription Save Folder",
	})
	if err != nil {
		return "", err
	}
	return path, nil
}

// EnsureSettingsVisible resizes the window to settingsH and, if the window
// would extend below the screen's working area, shifts it upward just enough
// to keep the entire panel on screen.
//
// screenTop and screenAvailH are passed in from the frontend (window.screen.availTop
// and window.screen.availHeight in the WebView), which are accurate for the
// monitor the window is currently on — avoiding the need to call ScreenGetAll
// whose struct layout varies across Wails patch versions.
func (a *App) EnsureSettingsVisible(settingsH, screenTop, screenAvailH int) error {
	x, y := runtime.WindowGetPosition(a.ctx)

	// Snapshot the original position so RestorePosition can return here on close.
	a.savedX = x
	a.savedY = y

	// Lowest Y the window can sit so its bottom stays within the available area
	maxY := screenTop + screenAvailH - settingsH
	if maxY < screenTop {
		maxY = screenTop
	}

	newY := y
	if y > maxY {
		newY = maxY
	}

	if newY != y {
		runtime.WindowSetPosition(a.ctx, x, newY)
	}
	runtime.WindowSetSize(a.ctx, 900, settingsH)
	return nil
}

// RestorePosition moves the window back to where it was before EnsureSettingsVisible
// shifted it, then resizes to the normal caption-bar height.
func (a *App) RestorePosition(normalH int) {
	runtime.WindowSetPosition(a.ctx, a.savedX, a.savedY)
	runtime.WindowSetSize(a.ctx, 900, normalH)
}

// MoveToBottom positions the window at the bottom of the available screen area
// after the onboarding overlay completes. screenTop and screenAvailH come from
// window.screen.availTop / availHeight in the WebView.
func (a *App) MoveToBottom(windowH, screenTop, screenAvailH int) {
	x, _ := runtime.WindowGetPosition(a.ctx)
	newY := screenTop + screenAvailH - windowH
	if newY < screenTop {
		newY = screenTop
	}
	runtime.WindowSetPosition(a.ctx, x, newY)
}