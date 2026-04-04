package app

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/asticode/go-astiav"
)

// startAstiavTranscode implements a native Go remuxing and transcoding pipeline
// via libavformat/libavcodec using the go-astiav wrapper, avoiding ffmpeg.exe
func startAstiavTranscode(ctx context.Context, nativePath string, startSeconds float64, windowSeconds float64, playbackMode VideoPlaybackMode) (io.ReadCloser, error) {
	pr, pw := io.Pipe()

	go func() {
		defer pw.Close()

		inputFormatContext := astiav.AllocFormatContext()
		if inputFormatContext == nil {
			pw.CloseWithError(fmt.Errorf("failed allocating input format context"))
			return
		}
		defer inputFormatContext.Free()

		if err := inputFormatContext.OpenInput(nativePath, nil, nil); err != nil {
			pw.CloseWithError(fmt.Errorf("failed opening input: %w", err))
			return
		}
		defer inputFormatContext.CloseInput()

		if playbackMode == VideoPlaybackModeTranscode {
			runVideoTranscodeLoop(ctx, inputFormatContext, startSeconds, windowSeconds, pw)
			return
		}
		remuxLoop(ctx, inputFormatContext, startSeconds, windowSeconds, pw)
	}()

	return pr, nil
}

// startAstiavTranscodeReader streams from an io.Reader (like an archive entry)
func startAstiavTranscodeReader(ctx context.Context, r io.Reader, playbackMode VideoPlaybackMode) (io.ReadCloser, error) {
	pr, pw := io.Pipe()

	go func() {
		defer pw.Close()

		inputFormatContext := astiav.AllocFormatContext()
		if inputFormatContext == nil {
			pw.CloseWithError(fmt.Errorf("failed allocating input format context"))
			return
		}
		defer inputFormatContext.Free()

		ioContext, err := astiav.AllocIOContext(
			131072, // 128KB buffer
			false,  // not writable
			func(b []byte) (int, error) { // read function
				return r.Read(b)
			},
			nil, // seek function
			nil, // write function
		)
		if err != nil {
			pw.CloseWithError(fmt.Errorf("failed allocating reader IO context: %w", err))
			return
		}
		defer ioContext.Free()

		inputFormatContext.SetPb(ioContext)

		if err := inputFormatContext.OpenInput("", nil, nil); err != nil {
			pw.CloseWithError(fmt.Errorf("failed opening input reader: %w", err))
			return
		}
		defer inputFormatContext.CloseInput()

		if playbackMode == VideoPlaybackModeTranscode {
			runVideoTranscodeLoop(ctx, inputFormatContext, 0, 0, pw)
			return
		}
		remuxLoop(ctx, inputFormatContext, 0, 0, pw)
	}()

	return pr, nil
}

func remuxLoop(ctx context.Context, inputFormatContext *astiav.FormatContext, startSeconds float64, windowSeconds float64, pw *io.PipeWriter) {
	if err := inputFormatContext.FindStreamInfo(nil); err != nil {
		pw.CloseWithError(fmt.Errorf("failed finding stream info: %w", err))
		return
	}

	if startSeconds > 0 {
		seekTarget := int64(startSeconds * float64(astiav.TimeBase))
		if err := inputFormatContext.SeekFrame(-1, seekTarget, astiav.NewSeekFlags(astiav.SeekFlagBackward)); err != nil {
			// Non-fatal seek error
		}
	}

	outputFormatContext, err := astiav.AllocOutputFormatContext(nil, "mp4", "")
	if err != nil {
		pw.CloseWithError(fmt.Errorf("failed allocating output format context: %w", err))
		return
	}
	defer outputFormatContext.Free()

	ioContext, err := astiav.AllocIOContext(
		4096,
		true,
		nil,
		nil,
		func(b []byte) (int, error) {
			n, err := pw.Write(b)
			return n, err
		},
	)
	if err != nil {
		pw.CloseWithError(fmt.Errorf("failed allocating custom IO context: %w", err))
		return
	}
	defer ioContext.Free()

	outputFormatContext.SetPb(ioContext)

	streamMapping := make(map[int]int)
	outputStreamIdx := 0
	selectedVideoStreamIndex := -1

	streamsArr := inputFormatContext.Streams()
	for i := 0; i < inputFormatContext.NbStreams(); i++ {
		inStream := streamsArr[i]
		codecParams := inStream.CodecParameters()
		if !shouldIncludePreviewStream(inStream, &selectedVideoStreamIndex) {
			streamMapping[i] = -1
			continue
		}

		outStream := outputFormatContext.NewStream(nil)
		if outStream == nil {
			pw.CloseWithError(fmt.Errorf("failed allocating output stream"))
			return
		}

		if err := codecParams.Copy(outStream.CodecParameters()); err != nil {
			pw.CloseWithError(fmt.Errorf("failed copying codec parameters: %w", err))
			return
		}
		outStream.CodecParameters().SetCodecTag(0)

		streamMapping[i] = outputStreamIdx
		outputStreamIdx++
	}

	options := astiav.NewDictionary()
	defer options.Free()
	options.Set("movflags", "frag_every_frame+empty_moov+default_base_moof+faststart+delay_moov", 0)

	if err := outputFormatContext.WriteHeader(options); err != nil {
		pw.CloseWithError(fmt.Errorf("failed writing header: %w", err))
		return
	}

	pkt := astiav.AllocPacket()
	if pkt == nil {
		pw.CloseWithError(fmt.Errorf("failed allocating packet"))
		return
	}
	defer pkt.Free()

	startTime := time.Now()

	for {
		select {
		case <-ctx.Done():
			outputFormatContext.WriteTrailer()
			return
		default:
		}

		if windowSeconds > 0 && time.Since(startTime).Seconds() > windowSeconds {
			break
		}

		if err := inputFormatContext.ReadFrame(pkt); err != nil {
			break // Assume EOF or read error
		}

		inStreamIdx := pkt.StreamIndex()
		outStreamIdx, ok := streamMapping[inStreamIdx]
		if !ok || outStreamIdx < 0 {
			pkt.Unref()
			continue
		}

		inStream := inputFormatContext.Streams()[inStreamIdx]
		outStream := outputFormatContext.Streams()[outStreamIdx]

		pkt.SetStreamIndex(outStreamIdx)
		pkt.RescaleTs(inStream.TimeBase(), outStream.TimeBase())
		pkt.SetPos(-1)

		if err := outputFormatContext.WriteInterleavedFrame(pkt); err != nil {
			pkt.Unref()
			pw.CloseWithError(fmt.Errorf("failed writing frame: %w", err))
			return
		}
		pkt.Unref()
	}

	if err := outputFormatContext.WriteTrailer(); err != nil {
		pw.CloseWithError(fmt.Errorf("failed writing trailer: %w", err))
		return
	}
}
