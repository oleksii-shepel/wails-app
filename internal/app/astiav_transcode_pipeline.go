package app

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/asticode/go-astiav"
)

type astiavVideoTranscodeSession struct {
	inputFormatContext  *astiav.FormatContext
	outputFormatContext *astiav.FormatContext
	outputIOContext     *astiav.IOContext
	inputStream         *astiav.Stream
	outputStream        *astiav.Stream
	decCodecContext     *astiav.CodecContext
	encCodecContext     *astiav.CodecContext
	decFrame            *astiav.Frame
	encPacket           *astiav.Packet
	scaleContext        *astiav.SoftwareScaleContext
	scaledFrame         *astiav.Frame
	lastDecodedPTS      *int64
}

func (s *astiavVideoTranscodeSession) close() {
	if s.scaleContext != nil {
		s.scaleContext.Free()
	}
	if s.scaledFrame != nil {
		s.scaledFrame.Free()
	}
	if s.encPacket != nil {
		s.encPacket.Free()
	}
	if s.decFrame != nil {
		s.decFrame.Free()
	}
	if s.encCodecContext != nil {
		s.encCodecContext.Free()
	}
	if s.decCodecContext != nil {
		s.decCodecContext.Free()
	}
	if s.outputIOContext != nil {
		s.outputIOContext.Free()
	}
	if s.outputFormatContext != nil {
		s.outputFormatContext.Free()
	}
}

func runVideoTranscodeLoop(ctx context.Context, inputFormatContext *astiav.FormatContext, startSeconds float64, windowSeconds float64, pw *io.PipeWriter) {
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

	session, err := newAstiavVideoTranscodeSession(inputFormatContext, pw)
	if err != nil {
		pw.CloseWithError(err)
		return
	}
	defer session.close()

	if err := session.run(ctx, windowSeconds); err != nil {
		pw.CloseWithError(err)
	}
}

func newAstiavVideoTranscodeSession(inputFormatContext *astiav.FormatContext, pw *io.PipeWriter) (*astiavVideoTranscodeSession, error) {
	session := &astiavVideoTranscodeSession{inputFormatContext: inputFormatContext}

	videoStream, err := findPrimaryAstiavVideoStream(inputFormatContext)
	if err != nil {
		return nil, err
	}
	session.inputStream = videoStream

	decoder := astiav.FindDecoder(videoStream.CodecParameters().CodecID())
	if decoder == nil {
		return nil, fmt.Errorf("failed finding decoder for video stream")
	}

	decCodecContext := astiav.AllocCodecContext(decoder)
	if decCodecContext == nil {
		return nil, fmt.Errorf("failed allocating decoder context")
	}
	session.decCodecContext = decCodecContext
	if err := videoStream.CodecParameters().ToCodecContext(decCodecContext); err != nil {
		return nil, fmt.Errorf("failed copying decoder params: %w", err)
	}
	decCodecContext.SetFramerate(inputFormatContext.GuessFrameRate(videoStream, nil))
	if err := decCodecContext.Open(decoder, nil); err != nil {
		return nil, fmt.Errorf("failed opening decoder: %w", err)
	}
	decCodecContext.SetTimeBase(videoStream.TimeBase())

	session.decFrame = astiav.AllocFrame()
	if session.decFrame == nil {
		return nil, fmt.Errorf("failed allocating decoder frame")
	}

	outputFormatContext, err := astiav.AllocOutputFormatContext(nil, "mp4", "")
	if err != nil {
		return nil, fmt.Errorf("failed allocating output format context: %w", err)
	}
	session.outputFormatContext = outputFormatContext

	outputIOContext, err := astiav.AllocIOContext(
		4096,
		true,
		nil,
		nil,
		func(b []byte) (int, error) {
			return pw.Write(b)
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed allocating custom IO context: %w", err)
	}
	session.outputIOContext = outputIOContext
	session.outputFormatContext.SetPb(outputIOContext)

	session.outputStream = session.outputFormatContext.NewStream(nil)
	if session.outputStream == nil {
		return nil, fmt.Errorf("failed allocating output video stream")
	}

	encoder := astiav.FindEncoder(astiav.CodecIDH264)
	if encoder == nil {
		return nil, fmt.Errorf("failed finding h264 encoder")
	}

	encCodecContext := astiav.AllocCodecContext(encoder)
	if encCodecContext == nil {
		return nil, fmt.Errorf("failed allocating encoder context")
	}
	session.encCodecContext = encCodecContext
	encCodecContext.SetHeight(decCodecContext.Height())
	encCodecContext.SetWidth(decCodecContext.Width())
	encCodecContext.SetPixelFormat(pickAstiavPixelFormat(encoder.SupportedPixelFormats(), decCodecContext.PixelFormat()))
	encCodecContext.SetSampleAspectRatio(decCodecContext.SampleAspectRatio())
	encCodecContext.SetTimeBase(pickAstiavTimeBase(decCodecContext.TimeBase(), videoStream.TimeBase(), astiav.NewRational(1, 30)))
	if session.outputFormatContext.OutputFormat().Flags().Has(astiav.IOFormatFlagGlobalheader) {
		encCodecContext.SetFlags(encCodecContext.Flags().Add(astiav.CodecContextFlagGlobalHeader))
	}

	encoderOptions := astiav.NewDictionary()
	defer encoderOptions.Free()
	encoderOptions.Set("preset", "ultrafast", 0)
	encoderOptions.Set("tune", "zerolatency", 0)
	if err := encCodecContext.Open(encoder, encoderOptions); err != nil {
		return nil, fmt.Errorf("failed opening h264 encoder: %w", err)
	}

	if err := session.outputStream.CodecParameters().FromCodecContext(encCodecContext); err != nil {
		return nil, fmt.Errorf("failed copying encoder params: %w", err)
	}
	session.outputStream.SetTimeBase(encCodecContext.TimeBase())

	session.encPacket = astiav.AllocPacket()
	if session.encPacket == nil {
		return nil, fmt.Errorf("failed allocating encoder packet")
	}

	if needsAstiavFrameScale(decCodecContext, encCodecContext) {
		scaleContext, err := astiav.CreateSoftwareScaleContext(
			decCodecContext.Width(), decCodecContext.Height(), decCodecContext.PixelFormat(),
			encCodecContext.Width(), encCodecContext.Height(), encCodecContext.PixelFormat(),
			astiav.SoftwareScaleContextFlags(astiav.SoftwareScaleContextFlagBilinear),
		)
		if err != nil {
			return nil, fmt.Errorf("failed creating scale context: %w", err)
		}
		session.scaleContext = scaleContext

		scaledFrame := astiav.AllocFrame()
		if scaledFrame == nil {
			return nil, fmt.Errorf("failed allocating scaled frame")
		}
		scaledFrame.SetWidth(encCodecContext.Width())
		scaledFrame.SetHeight(encCodecContext.Height())
		scaledFrame.SetPixelFormat(encCodecContext.PixelFormat())
		if err := scaledFrame.AllocBuffer(1); err != nil {
			return nil, fmt.Errorf("failed allocating scaled frame buffer: %w", err)
		}
		session.scaledFrame = scaledFrame
	}

	options := astiav.NewDictionary()
	defer options.Free()
	options.Set("movflags", "frag_every_frame+empty_moov+default_base_moof+faststart+delay_moov", 0)
	if err := session.outputFormatContext.WriteHeader(options); err != nil {
		return nil, fmt.Errorf("failed writing transcode header: %w", err)
	}

	return session, nil
}

func findPrimaryAstiavVideoStream(inputFormatContext *astiav.FormatContext) (*astiav.Stream, error) {
	for _, stream := range inputFormatContext.Streams() {
		if stream.CodecParameters().MediaType() == astiav.MediaTypeVideo {
			return stream, nil
		}
	}
	return nil, fmt.Errorf("no video stream found")
}

func pickAstiavPixelFormat(supported []astiav.PixelFormat, fallback astiav.PixelFormat) astiav.PixelFormat {
	if len(supported) > 0 {
		return supported[0]
	}
	return fallback
}

func pickAstiavTimeBase(values ...astiav.Rational) astiav.Rational {
	for _, value := range values {
		if value.Num() != 0 && value.Den() != 0 {
			return value
		}
	}
	return astiav.NewRational(1, 30)
}

func needsAstiavFrameScale(decoderContext *astiav.CodecContext, encoderContext *astiav.CodecContext) bool {
	return decoderContext.Width() != encoderContext.Width() ||
		decoderContext.Height() != encoderContext.Height() ||
		decoderContext.PixelFormat() != encoderContext.PixelFormat()
}

func (s *astiavVideoTranscodeSession) run(ctx context.Context, windowSeconds float64) error {
	packet := astiav.AllocPacket()
	if packet == nil {
		return fmt.Errorf("failed allocating packet")
	}
	defer packet.Free()

	firstVideoSeconds := -1.0

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := s.inputFormatContext.ReadFrame(packet); err != nil {
			if errors.Is(err, astiav.ErrEof) {
				break
			}
			return fmt.Errorf("failed reading frame: %w", err)
		}

		if packet.StreamIndex() != s.inputStream.Index() {
			packet.Unref()
			continue
		}

		if windowSeconds > 0 {
			if packetTimeSeconds, ok := packetTimestampSeconds(packet, s.inputStream); ok {
				if firstVideoSeconds < 0 {
					firstVideoSeconds = packetTimeSeconds
				}
				if packetTimeSeconds-firstVideoSeconds > windowSeconds {
					packet.Unref()
					break
				}
			}
		}

		packet.RescaleTs(s.inputStream.TimeBase(), s.decCodecContext.TimeBase())
		if err := s.decCodecContext.SendPacket(packet); err != nil {
			packet.Unref()
			return fmt.Errorf("failed sending packet to decoder: %w", err)
		}
		packet.Unref()

		if err := s.drainDecodedFrames(); err != nil {
			return err
		}
	}

	if err := s.decCodecContext.SendPacket(nil); err != nil && !errors.Is(err, astiav.ErrEof) {
		return fmt.Errorf("failed flushing decoder: %w", err)
	}
	if err := s.drainDecodedFrames(); err != nil {
		return err
	}
	if err := s.encCodecContext.SendFrame(nil); err != nil && !errors.Is(err, astiav.ErrEof) {
		return fmt.Errorf("failed flushing encoder: %w", err)
	}
	if err := s.drainEncodedPackets(); err != nil {
		return err
	}

	if err := s.outputFormatContext.WriteTrailer(); err != nil {
		return fmt.Errorf("failed writing transcode trailer: %w", err)
	}
	return nil
}

func (s *astiavVideoTranscodeSession) drainDecodedFrames() error {
	for {
		if err := s.decCodecContext.ReceiveFrame(s.decFrame); err != nil {
			if errors.Is(err, astiav.ErrEagain) || errors.Is(err, astiav.ErrEof) {
				return nil
			}
			return fmt.Errorf("failed receiving decoded frame: %w", err)
		}

		if s.lastDecodedPTS != nil && *s.lastDecodedPTS >= s.decFrame.Pts() {
			s.decFrame.Unref()
			continue
		}

		pts := s.decFrame.Pts()
		s.lastDecodedPTS = &pts

		frame, err := s.prepareFrameForEncode(s.decFrame)
		if err != nil {
			s.decFrame.Unref()
			return err
		}

		if err := s.encCodecContext.SendFrame(frame); err != nil {
			s.decFrame.Unref()
			return fmt.Errorf("failed sending frame to encoder: %w", err)
		}
		if err := s.drainEncodedPackets(); err != nil {
			s.decFrame.Unref()
			return err
		}
		s.decFrame.Unref()
	}
}

func (s *astiavVideoTranscodeSession) prepareFrameForEncode(frame *astiav.Frame) (*astiav.Frame, error) {
	if s.scaleContext == nil || s.scaledFrame == nil {
		return frame, nil
	}

	if err := s.scaleContext.ScaleFrame(frame, s.scaledFrame); err != nil {
		return nil, fmt.Errorf("failed scaling frame: %w", err)
	}
	s.scaledFrame.SetPts(frame.Pts())
	return s.scaledFrame, nil
}

func (s *astiavVideoTranscodeSession) drainEncodedPackets() error {
	for {
		if err := s.encCodecContext.ReceivePacket(s.encPacket); err != nil {
			if errors.Is(err, astiav.ErrEagain) || errors.Is(err, astiav.ErrEof) {
				return nil
			}
			return fmt.Errorf("failed receiving encoded packet: %w", err)
		}

		s.encPacket.SetStreamIndex(s.outputStream.Index())
		s.encPacket.RescaleTs(s.encCodecContext.TimeBase(), s.outputStream.TimeBase())
		if err := s.outputFormatContext.WriteInterleavedFrame(s.encPacket); err != nil {
			s.encPacket.Unref()
			return fmt.Errorf("failed writing encoded packet: %w", err)
		}
		s.encPacket.Unref()
	}
}
