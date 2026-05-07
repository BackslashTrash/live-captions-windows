package main

import (
	"archive/zip"
	"bytes"
	"context"
	"embed"
	"encoding/binary"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"live-captions/audio"
	"live-captions/overlay"

	vosk "github.com/alphacep/vosk-api/go"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend
var assets embed.FS

const sampleRate = 16000

var sttMutex sync.Mutex
var recognizer *vosk.VoskRecognizer
var model *vosk.VoskModel
var appCtx context.Context

type VoskPartial struct {
	Partial string `json:"partial"`
}

type VoskResult struct {
	Text string `json:"text"`
}

var voskModels = map[string]struct {
	Folder string
	URL    string
}{
	"English": {"vosk-model-small-en-us-0.15", "https://alphacephei.com/vosk/models/vosk-model-small-en-us-0.15.zip"},
	"Spanish": {"vosk-model-small-es-0.42", "https://alphacephei.com/vosk/models/vosk-model-small-es-0.42.zip"},
	"French":  {"vosk-model-small-fr-0.22", "https://alphacephei.com/vosk/models/vosk-model-small-fr-0.22.zip"},
	"German":  {"vosk-model-small-de-0.15", "https://alphacephei.com/vosk/models/vosk-model-small-de-0.15.zip"},
}

type progressWriter struct {
	total uint64
	seen  uint64
	last  int
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n := len(p)
	pw.seen += uint64(n)
	percent := int(float64(pw.seen) / float64(pw.total) * 100)
	if percent > pw.last {
		runtime.EventsEmit(appCtx, "download_progress", percent)
		pw.last = percent
	}
	return n, nil
}

func changeModel(modelPath string) error {
	sttMutex.Lock()
	defer sttMutex.Unlock()

	newModel, err := vosk.NewModel(modelPath)
	if err != nil {
		return err
	}
	newRec, err := vosk.NewRecognizer(newModel, sampleRate)
	if err != nil {
		newModel.Free()
		return err
	}

	if recognizer != nil {
		recognizer.Free()
	}
	if model != nil {
		model.Free()
	}

	model = newModel
	recognizer = newRec
	return nil
}

func extractZip(zipPath string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		fpath := filepath.Join("models", f.Name)
		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}
		os.MkdirAll(filepath.Dir(fpath), os.ModePerm)
		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			continue
		}
		rc, err := f.Open()
		if err == nil {
			io.Copy(outFile, rc)
			rc.Close()
		}
		outFile.Close()
	}
	return nil
}

func downloadAndExtract(lang string) error {
	info := voskModels[lang]
	os.MkdirAll("models", os.ModePerm)
	zipPath := filepath.Join("models", info.Folder+".zip")

	resp, err := http.Get(info.URL)
	if err != nil {
		return err
	}
	
	out, err := os.Create(zipPath)
	if err != nil {
		resp.Body.Close()
		return err
	}

	writer := &progressWriter{total: uint64(resp.ContentLength)}
	_, err = io.Copy(out, io.TeeReader(resp.Body, writer))
	out.Close()
	resp.Body.Close() 
	if err != nil {
		return err
	}

	runtime.EventsEmit(appCtx, "download_progress", 100) 
	err = extractZip(zipPath)
	if err != nil {
		return err
	}

	os.Remove(zipPath)
	return nil
}

func main() {
	vosk.SetLogLevel(-1)
	app := &overlay.App{}
	audioQueue := make(chan []float32, 50)

	if err := audio.StartLoopback(audioQueue); err != nil {
		log.Fatal("Failed to start audio capture:", err)
	}

	go func() {
		for chunk := range audioQueue {
			buf := new(bytes.Buffer)
			for _, f := range chunk {
				val := int16(f * 32767.0)
				binary.Write(buf, binary.LittleEndian, val)
			}
			audioBytes := buf.Bytes()

			sttMutex.Lock()
			if recognizer != nil {
				if recognizer.AcceptWaveform(audioBytes) == 1 {
					var vRes VoskResult
					json.Unmarshal([]byte(recognizer.Result()), &vRes)
					if vRes.Text != "" {
						app.EmitCaption("FINAL:" + vRes.Text)
					}
				} else {
					var vPartial VoskPartial
					json.Unmarshal([]byte(recognizer.PartialResult()), &vPartial)
					if vPartial.Partial != "" {
						app.EmitCaption("PARTIAL:" + vPartial.Partial)
					}
				}
			}
			sttMutex.Unlock()
		}
	}()

	err := wails.Run(&options.App{
		Title:            "Live Captions",
		Width:            900,
		Height:           110,
		AlwaysOnTop:      true,
		Frameless:        true,
		BackgroundColour: &options.RGBA{R: 0, G: 0, B: 0, A: 0},
		AssetServer: &assetserver.Options{Assets: assets},
		Windows: &windows.Options{
			WebviewIsTransparent: true,
			WindowIsTranslucent:  true,
			BackdropType:         windows.None,
		},
		OnStartup: func(ctx context.Context) {
			appCtx = ctx
			app.OnStartup(ctx)

			go func() {
				time.Sleep(500 * time.Millisecond)
				defaultLang := "English"
				info := voskModels[defaultLang]
				modelPath := filepath.Join("models", info.Folder)

				if _, err := os.Stat(modelPath); os.IsNotExist(err) {
					runtime.EventsEmit(ctx, "download_started", defaultLang)
					err := downloadAndExtract(defaultLang)
					if err == nil {
						changeModel(modelPath)
						runtime.EventsEmit(ctx, "model_ready", defaultLang)
					} else {
						runtime.EventsEmit(ctx, "download_error", "Failed to download default model.")
					}
				} else {
					changeModel(modelPath)
					runtime.EventsEmit(ctx, "model_ready", defaultLang)
				}
			}()

			runtime.EventsOn(ctx, "switch_language", func(optionalData ...interface{}) {
				if len(optionalData) > 0 {
					lang := optionalData[0].(string)
					go func() {
						info, exists := voskModels[lang]
						if !exists { return }
						
						modelPath := filepath.Join("models", info.Folder)

						if _, err := os.Stat(modelPath); os.IsNotExist(err) {
							runtime.EventsEmit(ctx, "download_started", lang)
							err := downloadAndExtract(lang)
							if err != nil {
								runtime.EventsEmit(ctx, "download_error", "Failed to download model.")
								return
							}
						}

						err := changeModel(modelPath)
						if err != nil {
							runtime.EventsEmit(ctx, "download_error", "Failed to load model into memory.")
							return
						}
						runtime.EventsEmit(ctx, "model_ready", lang)
					}()
				}
			})

			// THE FIX: Smart Folder Path Creation
			runtime.EventsOn(ctx, "save_transcript", func(optionalData ...interface{}) {
				if len(optionalData) >= 2 {
					folderPath := optionalData[0].(string)
					content := optionalData[1].(string)
					
					// If user left box blank, default to Documents/Transcriptions
					if folderPath == "" {
						docs, err := os.UserHomeDir()
						if err == nil {
							folderPath = filepath.Join(docs, "Documents", "Transcriptions")
						} else {
							folderPath = "Transcriptions" // Extreme failsafe
						}
					}

					currentTime := time.Now().Format("2006-01-02_15-04-05")
					fullPath := filepath.Join(folderPath, "Transcription_"+currentTime+".txt")
					
					// If the folder they typed doesn't exist, this automatically creates it!
					os.MkdirAll(folderPath, os.ModePerm)
					
					os.WriteFile(fullPath, []byte(content), 0644)
				}
			})
		},
		Bind: []interface{}{app},
	})

	if err != nil {
		log.Fatal(err)
	}
}