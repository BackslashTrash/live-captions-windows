package overlay

import (
    "context"
    "github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
    ctx context.Context
}

func (a *App) OnStartup(ctx context.Context) {
    a.ctx = ctx
}

// Called from main goroutine to push captions to frontend
func (a *App) EmitCaption(text string) {
    runtime.EventsEmit(a.ctx, "caption", text)
}