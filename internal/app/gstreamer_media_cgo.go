//go:build cgo

package app

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
)

func buildGStreamerFragmentedMP4(ctx context.Context, nativePath string, startSeconds float64, windowSeconds float64, playbackMode VideoPlaybackMode) ([]byte, error) {
	if err := ensureGStreamerRuntime(); err != nil {
		return nil, err
	}
	gstInitOnce.Do(func() {
		gst.Init(nil)
	})

	pipeline, sink, err := newTranscodePipeline(nativePath, playbackMode)
	if err != nil {
		return nil, err
	}
	defer shutdownGStreamerPipeline(pipeline)

	var out bytes.Buffer
	sink.SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: func(s *app.Sink) gst.FlowReturn {
			sample := s.PullSample()
			if sample == nil {
				return gst.FlowEOS
			}
			defer sample.Unref()
			buffer := sample.GetBuffer()
			if buffer == nil {
				return gst.FlowError
			}
			chunk := buffer.Bytes()
			if len(chunk) > 0 {
				out.Write(chunk)
			}
			return gst.FlowOK
		},
	})

	if err := pipeline.SetState(gst.StatePaused); err != nil {
		return nil, fmt.Errorf("failed to preroll gstreamer transcode pipeline: %w", err)
	}
	if stateChange, state := pipeline.GetState(gst.StatePaused, gst.ClockTime(20*time.Second)); stateChange == gst.StateChangeFailure || state != gst.StatePaused {
		return nil, fmt.Errorf("gstreamer transcode pipeline failed to enter paused state")
	}

	if startSeconds > 0 || windowSeconds > 0 {
		startTime := time.Duration(startSeconds * float64(time.Second))
		stopType := gst.SeekTypeNone
		stopValue := int64(-1)
		if windowSeconds > 0 {
			stopType = gst.SeekTypeSet
			stopValue = (startTime + time.Duration(windowSeconds*float64(time.Second))).Nanoseconds()
		}
		if ok := pipeline.SendEvent(gst.NewSeekEvent(
			1.0,
			gst.FormatTime,
			gst.SeekFlagFlush|gst.SeekFlagKeyUnit|gst.SeekFlagAccurate,
			gst.SeekTypeSet,
			startTime.Nanoseconds(),
			stopType,
			stopValue,
		)); !ok {
			return nil, fmt.Errorf("gstreamer transcode pipeline rejected seek")
		}
	}

	if err := pipeline.SetState(gst.StatePlaying); err != nil {
		return nil, fmt.Errorf("failed to start gstreamer transcode pipeline: %w", err)
	}

	bus := pipeline.GetPipelineBus()
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		msg := bus.TimedPop(gst.ClockTime(250 * time.Millisecond))
		if msg == nil {
			continue
		}

		switch msg.Type() {
		case gst.MessageEOS:
			if out.Len() == 0 {
				msg.Unref()
				return nil, fmt.Errorf("gstreamer produced no transcoded output")
			}
			msg.Unref()
			return out.Bytes(), nil
		case gst.MessageError:
			err := fmt.Errorf("gstreamer transcode failed: %w", msg.ParseError())
			msg.Unref()
			return nil, err
		default:
			msg.Unref()
		}
	}
}

func extractVideoFrameWithGStreamer(ctx context.Context, inputPath string, targetFormat string, frameSeconds float64) ([]byte, error) {
	if err := ensureGStreamerRuntime(); err != nil {
		return nil, err
	}
	gstInitOnce.Do(func() {
		gst.Init(nil)
	})

	encoder, mimeType, err := frameEncoderForFormat(targetFormat)
	if err != nil {
		return nil, err
	}
	_ = mimeType

	pipelineText := fmt.Sprintf(
		`filesrc location="%s" ! decodebin name=dec `+
			`dec. ! queue ! videoconvert ! %s ! appsink name=framesink sync=false async=false`,
		gstLaunchEscape(inputPath),
		encoder,
	)
	pipeline, err := gst.NewPipelineFromString(pipelineText)
	if err != nil {
		return nil, fmt.Errorf("failed to create gstreamer frame pipeline: %w", err)
	}
	defer shutdownGStreamerPipeline(pipeline)

	sinkElem, err := pipeline.GetElementByName("framesink")
	if err != nil {
		return nil, err
	}
	sink := app.SinkFromElement(sinkElem)
	if sink == nil {
		return nil, fmt.Errorf("failed to create frame appsink wrapper")
	}
	sink.SetMaxBuffers(1)
	sink.SetDrop(true)
	sink.SetWaitOnEOS(false)

	if err := pipeline.SetState(gst.StatePaused); err != nil {
		return nil, fmt.Errorf("failed to preroll gstreamer frame pipeline: %w", err)
	}
	if stateChange, state := pipeline.GetState(gst.StatePaused, gst.ClockTime(20*time.Second)); stateChange == gst.StateChangeFailure || state != gst.StatePaused {
		return nil, fmt.Errorf("gstreamer frame pipeline failed to enter paused state")
	}

	if frameSeconds < 0 {
		frameSeconds = 0
	}
	if ok := pipeline.SendEvent(gst.NewSeekEvent(
		1.0,
		gst.FormatTime,
		gst.SeekFlagFlush|gst.SeekFlagKeyUnit|gst.SeekFlagAccurate,
		gst.SeekTypeSet,
		time.Duration(frameSeconds*float64(time.Second)).Nanoseconds(),
		gst.SeekTypeNone,
		-1,
	)); !ok {
		return nil, fmt.Errorf("gstreamer frame pipeline rejected seek")
	}

	sample := sink.TryPullPreroll(gst.ClockTime(10 * time.Second))
	if sample == nil {
		if ctx != nil && ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("gstreamer produced no preview frame")
	}
	defer sample.Unref()
	buffer := sample.GetBuffer()
	if buffer == nil {
		return nil, fmt.Errorf("gstreamer produced no preview frame")
	}
	data := buffer.Bytes()
	if len(data) == 0 {
		return nil, fmt.Errorf("gstreamer produced no preview frame")
	}
	return data, nil
}

func newTranscodePipeline(nativePath string, playbackMode VideoPlaybackMode) (*gst.Pipeline, *app.Sink, error) {
	pipelineText := fmt.Sprintf(
		`filesrc location="%s" ! decodebin name=dec `+
			`dec. ! queue ! videoconvert ! x264enc tune=zerolatency speed-preset=ultrafast key-int-max=30 option-string="scenecut=0" ! h264parse ! mux. `+
			`dec. ! queue ! audioconvert ! audioresample ! voaacenc bitrate=128000 ! aacparse ! mux. `+
			`mp4mux name=mux fragment-duration=100 streamable=true ! appsink name=transcodesink sync=false async=false`,
		gstLaunchEscape(nativePath),
	)
	_ = playbackMode
	pipeline, err := gst.NewPipelineFromString(pipelineText)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create gstreamer transcode pipeline: %w", err)
	}
	sinkElem, err := pipeline.GetElementByName("transcodesink")
	if err != nil {
		return nil, nil, err
	}
	sink := app.SinkFromElement(sinkElem)
	if sink == nil {
		return nil, nil, fmt.Errorf("failed to create transcode appsink wrapper")
	}
	sink.SetArg("emit-signals", "true")
	sink.SetArg("max-buffers", "0")
	sink.SetArg("drop", "false")
	return pipeline, sink, nil
}

func frameEncoderForFormat(targetFormat string) (encoder string, mimeType string, err error) {
	switch normalizeFrameFormat(targetFormat) {
	case "png":
		return "pngenc snapshot=true", "image/png", nil
	case "jpg", "jpeg":
		return "jpegenc", "image/jpeg", nil
	default:
		return "", "", fmt.Errorf("unsupported frame target format: %s", targetFormat)
	}
}

func normalizeFrameFormat(value string) string {
	value = strings.ToLower(strings.TrimPrefix(value, "."))
	if value == "" {
		return "png"
	}
	return value
}

func shutdownGStreamerPipeline(pipeline *gst.Pipeline) {
	if pipeline == nil {
		return
	}
	_ = pipeline.SetState(gst.StateNull)
	_, _ = pipeline.GetState(gst.StateNull, gst.ClockTime(5*time.Second))
}
