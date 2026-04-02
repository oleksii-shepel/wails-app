//go:build cgo

package app

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"

	"wails-app/internal/utility"
)

var gstInitOnce sync.Once

func buildGStreamerSegmentedVideoFragment(ctx context.Context, nativePath string, startSeconds float64, durationSeconds float64) ([]byte, []byte, error) {
	if err := ensureGStreamerRuntime(); err != nil {
		return nil, nil, err
	}
	gstInitOnce.Do(func() {
		gst.Init(nil)
	})

	if durationSeconds <= 0 {
		return nil, nil, fmt.Errorf("invalid segment duration")
	}
	if startSeconds < 0 {
		startSeconds = 0
	}

	pipeline, sink, err := newSegmentEncodePipeline(nativePath, durationSeconds)
	if err != nil {
		return nil, nil, err
	}
	defer pipeline.SetState(gst.StateNull)

	var out bytes.Buffer
	var outMu sync.Mutex
	sink.SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: func(s *app.Sink) gst.FlowReturn {
			sample := s.PullSample()
			if sample == nil {
				return gst.FlowEOS
			}
			buffer := sample.GetBuffer()
			if buffer == nil {
				return gst.FlowError
			}
			chunk := buffer.Bytes()
			if len(chunk) == 0 {
				return gst.FlowOK
			}
			outMu.Lock()
			out.Write(chunk)
			outMu.Unlock()
			return gst.FlowOK
		},
	})

	if err := pipeline.SetState(gst.StatePaused); err != nil {
		return nil, nil, fmt.Errorf("failed to preroll gstreamer pipeline: %w", err)
	}
	if stateChange, state := pipeline.GetState(gst.StatePaused, gst.ClockTime(15*time.Second)); stateChange == gst.StateChangeFailure || state != gst.StatePaused {
		return nil, nil, fmt.Errorf("gstreamer pipeline failed to enter paused state")
	}

	startTime := time.Duration(startSeconds * float64(time.Second))
	stopTime := startTime + time.Duration(durationSeconds*float64(time.Second))
	seekFlags := gst.SeekFlagFlush | gst.SeekFlagKeyUnit | gst.SeekFlagAccurate
	seek := gst.NewSeekEvent(
		1.0,
		gst.FormatTime,
		seekFlags,
		gst.SeekTypeSet,
		startTime.Nanoseconds(),
		gst.SeekTypeSet,
		stopTime.Nanoseconds(),
	)
	if ok := pipeline.SendEvent(seek); !ok {
		return nil, nil, fmt.Errorf("gstreamer pipeline rejected segment seek")
	}

	if err := pipeline.SetState(gst.StatePlaying); err != nil {
		return nil, nil, fmt.Errorf("failed to start gstreamer pipeline: %w", err)
	}

	bus := pipeline.GetPipelineBus()
	tick := gst.ClockTime(250 * time.Millisecond)
	for {
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		default:
		}

		msg := bus.TimedPop(tick)
		if msg == nil {
			continue
		}

		switch msg.Type() {
		case gst.MessageEOS:
			data := out.Bytes()
			if len(data) == 0 {
				return nil, nil, fmt.Errorf("gstreamer produced no fragmented mp4 output")
			}
			initSegment, mediaSegment, err := splitFragmentedMP4(data)
			if err != nil {
				return nil, nil, err
			}
			return initSegment, mediaSegment, nil
		case gst.MessageError:
			return nil, nil, fmt.Errorf("gstreamer segment build failed: %w", msg.ParseError())
		}
	}
}

func ensureGStreamerRuntime() error {
	inspectPath, ok := utility.ResolveExecutable("gst-inspect-1.0", "gst-inspect-1.0.exe")
	if !ok {
		return fmt.Errorf("gstreamer is not installed or not available in the bundled tools directory")
	}

	root := filepath.Dir(filepath.Dir(inspectPath))
	binDir := filepath.Join(root, "bin")
	pluginDir := filepath.Join(root, "lib", "gstreamer-1.0")
	scannerPath := filepath.Join(root, "libexec", "gstreamer-1.0", scannerExecutableName())
	gioModules := filepath.Join(root, "lib", "gio", "modules")
	schemaDir := filepath.Join(root, "share", "glib-2.0", "schemas")

	prependUniqueEnvPath("PATH", binDir)
	setEnvIfDir("GST_PLUGIN_SYSTEM_PATH_1_0", pluginDir)
	setEnvIfFile("GST_PLUGIN_SCANNER", scannerPath)
	setEnvIfDir("GIO_EXTRA_MODULES", gioModules)
	setEnvIfDir("GSETTINGS_SCHEMA_DIR", schemaDir)
	return nil
}

func newSegmentEncodePipeline(nativePath string, durationSeconds float64) (*gst.Pipeline, *app.Sink, error) {
	fragmentMillis := int64(durationSeconds * 1000)
	if fragmentMillis <= 0 {
		fragmentMillis = int64(segmentedPreviewSegmentSeconds * 1000)
	}

	pipelineText := fmt.Sprintf(
		`filesrc location="%s" ! decodebin name=dec `+
			`dec. ! queue ! videoconvert ! x264enc tune=zerolatency speed-preset=ultrafast key-int-max=60 option-string="scenecut=0" ! h264parse ! mux. `+
			`dec. ! queue ! audioconvert ! audioresample ! voaacenc bitrate=128000 ! aacparse ! mux. `+
			`mp4mux name=mux fragment-duration=%d streamable=true ! appsink name=segmentsink sync=false async=false`,
		gstLaunchEscape(nativePath),
		fragmentMillis,
	)

	pipeline, err := gst.NewPipelineFromString(pipelineText)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create gstreamer segment pipeline: %w", err)
	}

	sinkElem, err := pipeline.GetElementByName("segmentsink")
	if err != nil {
		return nil, nil, err
	}
	sink := app.SinkFromElement(sinkElem)
	if sink == nil {
		return nil, nil, fmt.Errorf("failed to create appsink wrapper")
	}
	sink.SetArg("emit-signals", "true")
	sink.SetArg("max-buffers", "0")
	sink.SetArg("drop", "false")
	return pipeline, sink, nil
}

func prependUniqueEnvPath(name string, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	current := os.Getenv(name)
	parts := []string{}
	seen := map[string]struct{}{}
	appendPart := func(part string) {
		part = strings.TrimSpace(part)
		if part == "" {
			return
		}
		key := strings.ToLower(part)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		parts = append(parts, part)
	}
	appendPart(value)
	for _, part := range strings.Split(current, string(os.PathListSeparator)) {
		appendPart(part)
	}
	_ = os.Setenv(name, strings.Join(parts, string(os.PathListSeparator)))
}

func setEnvIfDir(name string, value string) {
	if info, err := os.Stat(value); err == nil && info.IsDir() {
		_ = os.Setenv(name, value)
	}
}

func setEnvIfFile(name string, value string) {
	if info, err := os.Stat(value); err == nil && !info.IsDir() {
		_ = os.Setenv(name, value)
	}
}

func gstLaunchEscape(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
	return replacer.Replace(value)
}

func scannerExecutableName() string {
	if runtime.GOOS == "windows" {
		return "gst-plugin-scanner.exe"
	}
	return "gst-plugin-scanner"
}
