package audio

import (
    "encoding/binary"
    "math"

    "github.com/gen2brain/malgo"
)

func StartLoopback(out chan<- []float32) error {
    ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
    if err != nil {
        return err
    }

    cfg := malgo.DefaultDeviceConfig(malgo.Loopback)
    cfg.Capture.Format   = malgo.FormatS16
    cfg.Capture.Channels = 1
    cfg.SampleRate       = 16000

    callbacks := malgo.DeviceCallbacks{
        Data: func(_, input []byte, _ uint32) {
            out <- int16ToFloat32(input)
        },
    }

    device, err := malgo.InitDevice(ctx.Context, cfg, callbacks)
    if err != nil {
        return err
    }

    return device.Start()
}

func int16ToFloat32(b []byte) []float32 {
    samples := len(b) / 2
    out := make([]float32, samples)
    for i := range out {
        s := int16(binary.LittleEndian.Uint16(b[i*2:]))
        out[i] = float32(s) / math.MaxInt16
    }
    return out
}