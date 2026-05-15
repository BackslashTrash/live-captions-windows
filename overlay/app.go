package overlay

import (
	"context"
	"live-captions/audio"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx          context.Context
	audioManager *audio.Manager
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