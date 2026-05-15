package audio

import (
	"encoding/binary"
	"math"
	"sync"
	"unsafe"

	"github.com/gen2brain/malgo"
)

const (
	targetSampleRate = 16000

	// Noise gate threshold (RMS). Only applied to microphone input.
	// 0.01 ≈ -40dB — filters breath/hiss without cutting quiet speech.
	// Raise to 0.02–0.03 in noisy environments.
	noiseGateThreshold = 0.01

	// How many consecutive silent chunks to still forward before stopping.
	// Lets Vosk finalize the last word cleanly before going quiet.
	silenceChunksBeforeStop = 8
)

type Manager struct {
	ctx          *malgo.AllocatedContext
	device       *malgo.Device
	devices      []malgo.DeviceInfo
	audioQueue   chan<- []float32
	mu           sync.Mutex
	CurrentIndex int
	IsMic        bool

	// Mic-only resampler state
	nativeSampleRate uint32
	resampleBuf      []float32
	silentChunks     int
}

func NewManager(out chan<- []float32) (*Manager, error) {
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
	if err != nil {
		return nil, err
	}
	return &Manager{
		ctx:        ctx,
		audioQueue: out,
	}, nil
}

func (m *Manager) GetMicrophones() ([]map[string]interface{}, error) {
	devices, err := m.ctx.Devices(malgo.Capture)
	if err != nil {
		return nil, err
	}
	m.devices = devices

	var result []map[string]interface{}
	for i, d := range devices {
		result = append(result, map[string]interface{}{
			"index": i,
			"name":  d.Name(),
		})
	}
	return result, nil
}

func (m *Manager) SwitchSource(useMic bool, micIndex int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.device != nil {
		m.device.Uninit()
		m.device = nil
	}

	m.IsMic = useMic
	m.CurrentIndex = micIndex
	m.resampleBuf = nil
	m.silentChunks = 0

	var device *malgo.Device
	var err error

	if useMic {
		device, err = m.initMicrophone(micIndex)
	} else {
		device, err = m.initLoopback()
	}

	if err != nil {
		return err
	}

	m.device = device
	return m.device.Start()
}

// initLoopback is the original unchanged path — it worked fine, don't touch it.
func (m *Manager) initLoopback() (*malgo.Device, error) {
	cfg := malgo.DefaultDeviceConfig(malgo.Loopback)
	cfg.Capture.Format = malgo.FormatS16
	cfg.Capture.Channels = 1
	cfg.SampleRate = targetSampleRate

	callbacks := malgo.DeviceCallbacks{
		Data: func(_, input []byte, _ uint32) {
			m.audioQueue <- int16SliceToFloat32(input)
		},
	}

	return malgo.InitDevice(m.ctx.Context, cfg, callbacks)
}

// initMicrophone captures at the device's native rate and runs the full
// improvement pipeline: resample → normalize → noise gate.
func (m *Manager) initMicrophone(micIndex int) (*malgo.Device, error) {
	nativeRate := m.detectNativeSampleRate(micIndex)
	m.nativeSampleRate = nativeRate

	cfg := malgo.DefaultDeviceConfig(malgo.Capture)
	cfg.Capture.Format = malgo.FormatS16
	cfg.Capture.Channels = 1
	cfg.SampleRate = nativeRate

	if micIndex >= 0 && micIndex < len(m.devices) {
		cfg.Capture.DeviceID = unsafe.Pointer(&m.devices[micIndex].ID)
	}

	callbacks := malgo.DeviceCallbacks{
		Data: func(_, input []byte, _ uint32) {
			samples := int16SliceToFloat32(input)

			// 1. Resample from native rate to 16000 Hz
			resampled := m.resample(samples, nativeRate, targetSampleRate)
			if len(resampled) == 0 {
				return
			}

			// 2. Normalize amplitude to compensate for low mic gain
			resampled = normalize(resampled)

			// 3. Noise gate — skip silent chunks, allow tail for Vosk finalization
			rms := computeRMS(resampled)
			if rms < noiseGateThreshold {
				m.silentChunks++
				if m.silentChunks > silenceChunksBeforeStop {
					return
				}
			} else {
				m.silentChunks = 0
			}

			m.audioQueue <- resampled
		},
	}

	dev, err := malgo.InitDevice(m.ctx.Context, cfg, callbacks)
	if err != nil {
		// Fallback: if native rate detection failed, let malgo handle it at 16000
		m.nativeSampleRate = targetSampleRate
		cfg.SampleRate = targetSampleRate
		return malgo.InitDevice(m.ctx.Context, cfg, malgo.DeviceCallbacks{
			Data: func(_, input []byte, _ uint32) {
				samples := int16SliceToFloat32(input)
				samples = normalize(samples)
				rms := computeRMS(samples)
				if rms < noiseGateThreshold {
					m.silentChunks++
					if m.silentChunks > silenceChunksBeforeStop {
						return
					}
				} else {
					m.silentChunks = 0
				}
				m.audioQueue <- samples
			},
		})
	}
	return dev, nil
}

// detectNativeSampleRate probes the mic for its preferred rate.
// Only called for microphone mode, never for loopback.
func (m *Manager) detectNativeSampleRate(micIndex int) uint32 {
	commonRates := []uint32{48000, 44100, 96000, 32000, 22050, 16000}

	for _, rate := range commonRates {
		cfg := malgo.DefaultDeviceConfig(malgo.Capture)
		cfg.Capture.Format = malgo.FormatS16
		cfg.Capture.Channels = 1
		cfg.SampleRate = rate

		if micIndex >= 0 && micIndex < len(m.devices) {
			cfg.Capture.DeviceID = unsafe.Pointer(&m.devices[micIndex].ID)
		}

		dev, err := malgo.InitDevice(m.ctx.Context, cfg, malgo.DeviceCallbacks{})
		if err == nil {
			dev.Uninit()
			return rate
		}
	}

	return 48000
}

// resample converts float32 PCM from srcRate to dstRate using linear
// interpolation with fractional carry-over between chunks.
func (m *Manager) resample(input []float32, srcRate, dstRate uint32) []float32 {
	if srcRate == dstRate {
		return input
	}

	ratio := float64(srcRate) / float64(dstRate)
	src := append(m.resampleBuf, input...)

	outputLen := int(float64(len(input)) / ratio)
	if outputLen <= 0 {
		m.resampleBuf = src
		return nil
	}

	out := make([]float32, 0, outputLen)
	var pos float64
	for {
		iPos := int(pos)
		if iPos+1 >= len(src) {
			break
		}
		frac := float32(pos - float64(iPos))
		out = append(out, src[iPos]*(1-frac)+src[iPos+1]*frac)
		pos += ratio
	}

	iPos := int(pos)
	if iPos < len(src) {
		m.resampleBuf = src[iPos:]
	} else {
		m.resampleBuf = nil
	}

	return out
}

// normalize scales the chunk so its peak sits near 0.9.
// Capped at 8x gain so silence doesn't get blasted.
func normalize(samples []float32) []float32 {
	if len(samples) == 0 {
		return samples
	}
	var peak float32
	for _, s := range samples {
		if a := float32(math.Abs(float64(s))); a > peak {
			peak = a
		}
	}
	if peak < 0.001 {
		return samples
	}
	gain := float32(0.9) / peak
	if gain > 8.0 {
		gain = 8.0
	}
	out := make([]float32, len(samples))
	for i, s := range samples {
		v := s * gain
		if v > 1.0 {
			v = 1.0
		} else if v < -1.0 {
			v = -1.0
		}
		out[i] = v
	}
	return out
}

// computeRMS returns the root-mean-square energy of a chunk.
func computeRMS(samples []float32) float32 {
	if len(samples) == 0 {
		return 0
	}
	var sum float64
	for _, s := range samples {
		sum += float64(s) * float64(s)
	}
	return float32(math.Sqrt(sum / float64(len(samples))))
}

// int16SliceToFloat32 converts raw S16LE bytes to normalized float32 [-1, 1].
func int16SliceToFloat32(b []byte) []float32 {
	samples := len(b) / 2
	out := make([]float32, samples)
	for i := range out {
		s := int16(binary.LittleEndian.Uint16(b[i*2:]))
		out[i] = float32(s) / math.MaxInt16
	}
	return out
}