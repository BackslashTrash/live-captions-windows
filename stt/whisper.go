package stt

import (
    "strings"
    whisper "github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
)

type Engine struct {
    model   whisper.Model
    context whisper.Context
}

func New(modelPath string) (*Engine, error) {
    model, err := whisper.New(modelPath)
    if err != nil {
        return nil, err
    }
    ctx, err := model.NewContext()
    if err != nil {
        return nil, err
    }
    ctx.SetLanguage("en")
    ctx.SetBeamSize(1)
    return &Engine{model: model, context: ctx}, nil
}

func (e *Engine) Transcribe(samples []float32) (string, error) {
    var sb strings.Builder
    err := e.context.Process(
        samples,
        nil,
        func(seg whisper.Segment) {
            sb.WriteString(seg.Text)
        },
        nil,
    )
    if err != nil {
        return "", err
    }
    return strings.TrimSpace(sb.String()), nil
}

func (e *Engine) Close() {
    e.model.Close()
}