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
	if ctx == nil {
		ctx = context.Background()
	}
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
	defer shutdownGStreamerPipeline(pipeline)

	var out bytes.Buffer
	var outMu sync.Mutex
	sink.SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: func(s *app.Sink) gst.FlowReturn {
			sample := s.PullSample()
			if sample == nil {
				return gst.FlowEOS
			}
			defer releaseGstSample(sample)
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
	if stateChange, state := pipeline.GetState(gst.StatePaused, gstClockTime(15*time.Second)); stateChange == gst.StateChangeFailure || state != gst.StatePaused {
		return nil, nil, fmt.Errorf("gstreamer pipeline failed to enter paused state")
	}

	startTime := time.Duration(startSeconds * float64(time.Second))
	stopTime := startTime + time.Duration(durationSeconds*float64(time.Second))
	// We MUST NOT use gst.SeekFlagKeyUnit here! Using it causes GStreamer to snap the start time
	// backwards to the nearest I-frame, which sends past frames to x264enc. Since mp4mux resets its
	// timestamp to 0.0 for every chunk, the frontend adds the requested segment offset (e.g. 5.0s),
	// which shifts these snapped past frames forward, creating visually repetitive frames at the boundary.
	//
	// SeekFlagSnapAfter is also harmful: it skips forward to the next keyframe, which creates gaps
	// (missing frames) between segments. The frontend expects contiguous coverage from each segment's
	// startTime to startTime+duration. When SnapAfter skips past that boundary, the segment delivers
	// fewer frames than expected, creating frozen/missing ranges (e.g. freezes at 1s, 3s, 7s for FLV).
	//
	// Accurate seek is correct for ALL formats. Any surplus pre-seek frames that flvdemux/other demuxers
	// emit are stripped by the frontend's trimPacketsToSegmentWindow + duplicate timestamp dedup.
	seekFlags := gst.SeekFlagFlush | gst.SeekFlagAccurate
	seek := gst.NewSeekEvent(
		1.0,
		gst.FormatTime,
		seekFlags,
		gst.SeekTypeSet,
		startTime.Nanoseconds(),
		gst.SeekTypeSet,
		stopTime.Nanoseconds(),
	)

	dec, err := pipeline.GetElementByName("dec")
	var ok bool
	if err == nil && dec != nil {
		ok = dec.SendEvent(seek)
	}
	if !ok {
		ok = pipeline.SendEvent(seek)
	}
	if !ok {
		return nil, nil, fmt.Errorf("gstreamer pipeline rejected segment seek")
	}

	// Install pad probes to clip any pre-seek frames that demuxers like flvdemux
	// emit when they can't honor accurate seek properly. Without this, frames from
	// the previous keyframe (before startTime) leak through with running time 0,
	// causing the frontend to display wrong content in every segment.
	if startSeconds > 0 {
		clipNs := gst.ClockTime(startTime.Nanoseconds())
		installSegmentClipProbe(pipeline, "vqueue", clipNs)
		installSegmentClipProbe(pipeline, "aqueue", clipNs)
	}

	// Strip any preroll frames (moof/mdat) generated before the seek, keeping only initialization boxes (ftyp/moov).
	outMu.Lock()
	prerollData := out.Bytes()
	if initSegment, _, err := splitFragmentedMP4(prerollData); err == nil && len(initSegment) > 0 {
		out.Reset()
		out.Write(initSegment)
	} else if len(prerollData) > 0 {
		// If split fails, mp4mux might not have pushed moof yet (only ftyp/moov). Leave it intact.
	}
	outMu.Unlock()

	if err := pipeline.SetState(gst.StatePlaying); err != nil {
		return nil, nil, fmt.Errorf("failed to start gstreamer pipeline: %w", err)
	}

	bus := pipeline.GetPipelineBus()
	tick := gstClockTime(250 * time.Millisecond)
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

		msgType := msg.Type()
		if msgType == gst.MessageEOS {
			outMu.Lock()
			data := append([]byte(nil), out.Bytes()...)
			outMu.Unlock()
			if len(data) == 0 {
				releaseGstMessage(msg)
				return nil, nil, fmt.Errorf("gstreamer produced no fragmented mp4 output")
			}
			initSegment, mediaSegment, err := splitFragmentedMP4(data)
			if err != nil {
				releaseGstMessage(msg)
				return nil, nil, err
			}
			releaseGstMessage(msg)
			return initSegment, mediaSegment, nil
		}
		if msgType == gst.MessageError {
			err := fmt.Errorf("gstreamer segment build failed: %w", msg.ParseError())
			releaseGstMessage(msg)
			return nil, nil, err
		}
		releaseGstMessage(msg)
	}
}

// installSegmentClipProbe adds a pad probe on the src pad of the named element
// that drops any buffer with PTS strictly before clipPTS. This prevents demuxers
// like flvdemux from leaking pre-seek keyframe-anchored frames into the encoded
// output. The downstream videorate element fills any resulting gap at the start
// of the segment by duplicating the first valid frame.
func installSegmentClipProbe(pipeline *gst.Pipeline, elementName string, clipPTS gst.ClockTime) {
	elem, err := pipeline.GetElementByName(elementName)
	if err != nil || elem == nil {
		return
	}
	srcPad := elem.GetStaticPad("src")
	if srcPad == nil {
		return
	}
	srcPad.AddProbe(gst.PadProbeTypeBuffer, func(self *gst.Pad, info *gst.PadProbeInfo) gst.PadProbeReturn {
		buf := info.GetBuffer()
		if buf == nil {
			return gst.PadProbeOK
		}
		pts := buf.PresentationTimestamp()
		if pts == gst.ClockTimeNone {
			return gst.PadProbeOK
		}
		if pts < clipPTS {
			return gst.PadProbeDrop
		}
		// All subsequent buffers should be >= clipPTS, remove the probe
		return gst.PadProbeRemove
	})
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

	var hasVideo, hasAudio bool
	if probe, err := probeVideo(nativePath); err == nil && probe != nil {
		for _, stream := range probe.Streams {
			if stream.CodecType == "video" {
				hasVideo = true
			}
			if stream.CodecType == "audio" {
				hasAudio = true
			}
		}
	}
	if !hasVideo && !hasAudio {
		// Fallback if probe failed or returned zero streams
		hasVideo = true
		hasAudio = true
	}

	pipelineText := fmt.Sprintf(`filesrc location="%s" ! decodebin name=dec `, gstLaunchEscape(nativePath))
	if hasVideo {
		pipelineText += `dec. ! queue name=vqueue ! videoconvert ! videorate ! x264enc tune=zerolatency speed-preset=ultrafast key-int-max=120 option-string="scenecut=0" ! h264parse ! mux. `
	}
	if hasAudio {
		pipelineText += `dec. ! queue name=aqueue ! audioconvert ! audioresample ! audiorate ! voaacenc bitrate=128000 ! aacparse ! mux. `
	}
	pipelineText += fmt.Sprintf(`mp4mux name=mux fragment-duration=%d streamable=true ! appsink name=segmentsink sync=false async=false`, fragmentMillis)

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
