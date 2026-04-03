//go:build cgo

package app

import (
	"bytes"
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/go-gst/go-gst/gst"
	"github.com/go-gst/go-gst/gst/app"
)

func buildGStreamerFragmentedMP4(ctx context.Context, nativePath string, startSeconds float64, windowSeconds float64, playbackMode VideoPlaybackMode) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
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
			if len(chunk) > 0 {
				outMu.Lock()
				out.Write(chunk)
				outMu.Unlock()
			}
			return gst.FlowOK
		},
	})

	if err := pipeline.SetState(gst.StatePaused); err != nil {
		return nil, fmt.Errorf("failed to preroll gstreamer transcode pipeline: %w", err)
	}
	if stateChange, state := pipeline.GetState(gst.StatePaused, gstClockTime(20*time.Second)); stateChange == gst.StateChangeFailure || state != gst.StatePaused {
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

		msg := bus.TimedPop(gstClockTime(250 * time.Millisecond))
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
				return nil, fmt.Errorf("gstreamer produced no transcoded output")
			}
			releaseGstMessage(msg)
			return data, nil
		}
		if msgType == gst.MessageError {
			err := fmt.Errorf("gstreamer transcode failed: %w", msg.ParseError())
			releaseGstMessage(msg)
			return nil, err
		}
		releaseGstMessage(msg)
	}
}

func extractVideoFrameWithGStreamer(ctx context.Context, inputPath string, targetFormat string, frameSeconds float64) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
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
	if stateChange, state := pipeline.GetState(gst.StatePaused, gstClockTime(20*time.Second)); stateChange == gst.StateChangeFailure || state != gst.StatePaused {
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

	if err := pipeline.SetState(gst.StatePlaying); err != nil {
		return nil, fmt.Errorf("failed to start gstreamer frame pipeline: %w", err)
	}

	sample, err := pullFrameSample(ctx, pipeline, sink, 10*time.Second)
	if err != nil {
		return nil, err
	}
	if sample == nil {
		return nil, fmt.Errorf("gstreamer produced no preview frame")
	}
	defer releaseGstSample(sample)
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

func pullFrameSample(ctx context.Context, pipeline *gst.Pipeline, sink *app.Sink, timeout time.Duration) (*gst.Sample, error) {
	deadline := time.Now().Add(timeout)
	bus := pipeline.GetPipelineBus()
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if remaining := time.Until(deadline); remaining <= 0 {
			return nil, nil
		}

		sample := sink.TryPullSample(gstClockTime(100 * time.Millisecond))
		if sample != nil {
			return sample, nil
		}

		for {
			msg := bus.TimedPop(gstClockTime(0))
			if msg == nil {
				break
			}
			msgType := msg.Type()
			if msgType == gst.MessageError {
				err := fmt.Errorf("gstreamer frame extraction failed: %w", msg.ParseError())
				releaseGstMessage(msg)
				return nil, err
			}
			if msgType == gst.MessageEOS {
				releaseGstMessage(msg)
				return nil, fmt.Errorf("gstreamer produced no preview frame")
			}
			releaseGstMessage(msg)
		}
	}
}

func shutdownGStreamerPipeline(pipeline *gst.Pipeline) {
	if pipeline == nil {
		return
	}
	_ = pipeline.SetState(gst.StateNull)
	_, _ = pipeline.GetState(gst.StateNull, gstClockTime(5*time.Second))
}

func gstClockTime(duration time.Duration) gst.ClockTime {
	return *(*gst.ClockTime)(unsafe.Pointer(&duration))
}

func releaseGstSample(sample *gst.Sample) {
	if sample == nil {
		return
	}
	runtime.SetFinalizer(sample, nil)
	sample.Unref()
}

func releaseGstMessage(message *gst.Message) {
	if message == nil {
		return
	}
	runtime.SetFinalizer(message, nil)
	message.Unref()
}
