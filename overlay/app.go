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