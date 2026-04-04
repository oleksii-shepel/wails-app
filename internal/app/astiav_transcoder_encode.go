package app

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/asticode/go-astiav"
	"github.com/asticode/go-astikit"
)

type transcodeStream struct {
	buffersinkContext   *astiav.BuffersinkFilterContext
	buffersrcContext    *astiav.BuffersrcFilterContext
	decCodec            *astiav.Codec
	decCodecContext     *astiav.CodecContext
	decFrame            *astiav.Frame
	decLastPTS          *int64
	encCodec            *astiav.Codec
	encCodecContext     *astiav.CodecContext
	encPkt              *astiav.Packet
	filterFrame         *astiav.Frame
	filterGraph         *astiav.FilterGraph
	inputStream         *astiav.Stream
	nextFramePTS        int64
	nextPacketDTS       int64
	segmentStartSeconds float64
	segmentEndSeconds   float64
	segmentStartPTS     int64
	encLastPTS          *int64
	outputStream        *astiav.Stream
	done                bool
}

// startAstiavEncodeTranscode opens a file and transcodes to fMP4 (H.264/AAC).
func startAstiavEncodeTranscode(ctx context.Context, nativePath string, startSeconds float64, windowSeconds float64, playbackMode VideoPlaybackMode) (io.ReadCloser, error) {
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

		transcodeLoop(ctx, inputFormatContext, startSeconds, windowSeconds, pw)
	}()

	return pr, nil
}

// startAstiavEncodeTranscodeReader transcodes from an io.Reader to fMP4.
func startAstiavEncodeTranscodeReader(ctx context.Context, r io.Reader, playbackMode VideoPlaybackMode) (io.ReadCloser, error) {
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
			131072,
			false,
			func(b []byte) (int, error) {
				return r.Read(b)
			},
			nil,
			nil,
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

		transcodeLoop(ctx, inputFormatContext, 0, 0, pw)
	}()

	return pr, nil
}

func transcodeLoop(ctx context.Context, inputFormatContext *astiav.FormatContext, startSeconds float64, windowSeconds float64, pw *io.PipeWriter) {
	if err := inputFormatContext.FindStreamInfo(nil); err != nil {
		pw.CloseWithError(fmt.Errorf("failed finding stream info: %w", err))
		return
	}
	transcodeLoopInner(ctx, inputFormatContext, startSeconds, windowSeconds, pw)
}

// transcodeLoopPreOpened is like transcodeLoop but assumes FindStreamInfo was already called.
func transcodeLoopPreOpened(ctx context.Context, inputFormatContext *astiav.FormatContext, startSeconds float64, windowSeconds float64, pw *io.PipeWriter) {
	transcodeLoopInner(ctx, inputFormatContext, startSeconds, windowSeconds, pw)
}

func transcodeLoopInner(ctx context.Context, inputFormatContext *astiav.FormatContext, startSeconds float64, windowSeconds float64, pw *io.PipeWriter) {
	c := astikit.NewCloser()
	defer c.Close()

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
	c.Add(outputFormatContext.Free)

	ioContext, err := astiav.AllocIOContext(
		4096,
		true,
		nil,
		nil,
		func(b []byte) (int, error) {
			n, wErr := pw.Write(b)
			return n, wErr
		},
	)
	if err != nil {
		pw.CloseWithError(fmt.Errorf("failed allocating custom IO context: %w", err))
		return
	}
	c.Add(ioContext.Free)
	outputFormatContext.SetPb(ioContext)

	streams := make(map[int]*transcodeStream)

	for _, is := range inputFormatContext.Streams() {
		if !isSegmentableStream(is) {
			continue
		}

		s := &transcodeStream{inputStream: is, segmentStartSeconds: startSeconds, segmentStartPTS: -1}
		if windowSeconds > 0 {
			s.segmentEndSeconds = startSeconds + windowSeconds
		}
		streams[is.Index()] = s

		if s.decCodec = astiav.FindDecoder(is.CodecParameters().CodecID()); s.decCodec == nil {
			pw.CloseWithError(fmt.Errorf("decoder not found for codec %s", is.CodecParameters().CodecID()))
			return
		}

		if s.decCodecContext = astiav.AllocCodecContext(s.decCodec); s.decCodecContext == nil {
			pw.CloseWithError(fmt.Errorf("failed allocating dec context"))
			return
		}
		c.Add(s.decCodecContext.Free)

		if err = is.CodecParameters().ToCodecContext(s.decCodecContext); err != nil {
			pw.CloseWithError(fmt.Errorf("failed copying props to dec: %w", err))
			return
		}

		if err = s.decCodecContext.Open(s.decCodec, nil); err != nil {
			pw.CloseWithError(fmt.Errorf("failed opening dec: %w", err))
			return
		}

		s.outputStream = outputFormatContext.NewStream(nil)
		if s.outputStream == nil {
			pw.CloseWithError(fmt.Errorf("failed allocating out stream"))
			return
		}

		codecID := astiav.CodecIDH264
		if s.decCodecContext.MediaType() == astiav.MediaTypeAudio {
			codecID = astiav.CodecIDAac
		}

		if s.encCodec = astiav.FindEncoder(codecID); s.encCodec == nil {
			pw.CloseWithError(fmt.Errorf("encoder not found for %s", codecID))
			return
		}

		if s.encCodecContext = astiav.AllocCodecContext(s.encCodec); s.encCodecContext == nil {
			pw.CloseWithError(fmt.Errorf("failed allocating enc context"))
			return
		}
		c.Add(s.encCodecContext.Free)

		if s.decCodecContext.MediaType() == astiav.MediaTypeAudio {
			if v := s.encCodec.SupportedChannelLayouts(); len(v) > 0 {
				s.encCodecContext.SetChannelLayout(v[0])
			} else {
				s.encCodecContext.SetChannelLayout(s.decCodecContext.ChannelLayout())
			}
			s.encCodecContext.SetSampleRate(s.decCodecContext.SampleRate())
			if v := s.encCodec.SupportedSampleFormats(); len(v) > 0 {
				s.encCodecContext.SetSampleFormat(v[0])
			} else {
				s.encCodecContext.SetSampleFormat(s.decCodecContext.SampleFormat())
			}
			s.encCodecContext.SetTimeBase(astiav.NewRational(1, s.encCodecContext.SampleRate()))
		} else {
			s.encCodecContext.SetHeight(s.decCodecContext.Height())
			s.encCodecContext.SetWidth(s.decCodecContext.Width())
			if v := s.encCodec.SupportedPixelFormats(); len(v) > 0 {
				s.encCodecContext.SetPixelFormat(v[0])
			} else {
				s.encCodecContext.SetPixelFormat(s.decCodecContext.PixelFormat())
			}
			s.encCodecContext.SetSampleAspectRatio(s.decCodecContext.SampleAspectRatio())
			// Use input framerate if available, else default to 25fps.
			fr := s.inputStream.AvgFrameRate()
			if fr.Num() <= 0 || fr.Den() <= 0 {
				fr = astiav.NewRational(25, 1)
			}
			s.encCodecContext.SetTimeBase(astiav.NewRational(fr.Den(), fr.Num()))
			s.encCodecContext.SetFramerate(fr)
		}

		if outputFormatContext.OutputFormat().Flags().Has(astiav.IOFormatFlagGlobalheader) {
			s.encCodecContext.SetFlags(s.encCodecContext.Flags().Add(astiav.CodecContextFlagGlobalHeader))
		}

		if s.decCodecContext.MediaType() == astiav.MediaTypeVideo {
			s.encCodecContext.SetGopSize(30)
		}

		var encOpts *astiav.Dictionary
		if codecID == astiav.CodecIDH264 {
			encOpts = astiav.NewDictionary()
			c.Add(encOpts.Free)
			encOpts.Set("preset", "ultrafast", 0)
			encOpts.Set("tune", "zerolatency", 0)
			encOpts.Set("crf", "23", 0)
		}

		if err = s.encCodecContext.Open(s.encCodec, encOpts); err != nil {
			pw.CloseWithError(fmt.Errorf("failed opening encoder: %w", err))
			return
		}

		if err = s.outputStream.CodecParameters().FromCodecContext(s.encCodecContext); err != nil {
			pw.CloseWithError(fmt.Errorf("failed creating codec params: %w", err))
			return
		}
		s.outputStream.SetTimeBase(s.encCodecContext.TimeBase())

		s.decFrame = astiav.AllocFrame()
		c.Add(s.decFrame.Free)
	}

	opts := astiav.NewDictionary()
	opts.Set("movflags", "frag_keyframe+empty_moov+default_base_moof", 0)
	c.Add(opts.Free)

	if err = outputFormatContext.WriteHeader(opts); err != nil {
		pw.CloseWithError(fmt.Errorf("failed writing header: %w", err))
		return
	}

	if err := initTranscodeFilters(streams, c); err != nil {
		pw.CloseWithError(err)
		return
	}

	pkt := astiav.AllocPacket()
	c.Add(pkt.Free)

	for {
		select {
		case <-ctx.Done():
			goto cleanup
		default:
		}

		if err := inputFormatContext.ReadFrame(pkt); err != nil {
			if errors.Is(err, astiav.ErrEof) {
				break
			}
			pw.CloseWithError(fmt.Errorf("reading frame failed: %w", err))
			return
		}

		s, ok := streams[pkt.StreamIndex()]
		if !ok {
			pkt.Unref()
			continue
		}

		// Early exit: check packet timestamp against segment window before
		// decoding. Without this, the loop reads the entire remaining file
		// even though filterEncodeWriteFrame silently drops late frames.
		if s.segmentEndSeconds > 0 && !s.done {
			rawPTS := pkt.Pts()
			if rawPTS == astiav.NoPtsValue {
				rawPTS = pkt.Dts()
			}
			if rawPTS != astiav.NoPtsValue {
				pktTime := timestampSeconds(rawPTS, s.inputStream.TimeBase())
				if pktTime >= s.segmentEndSeconds {
					s.done = true
				}
			}
		}
		if s.done {
			pkt.Unref()
			allDone := true
			for _, st := range streams {
				if !st.done {
					allDone = false
					break
				}
			}
			if allDone {
				break
			}
			continue
		}

		// Do NOT rescale PTS from stream TB to decoder TB here.
		// The decoder passes through PTS values opaquely, and the buffersrc
		// filter is configured with the stream's time base. Rescaling to the
		// decoder's time base (which may be 0/1 for H.264) corrupts
		// timestamps, causing filterEncodeWriteFrame to drop all frames for
		// non-zero segment starts.
		if err := s.decCodecContext.SendPacket(pkt); err != nil {
			pkt.Unref()
			continue // skip undecodable packets
		}

		pkt.Unref()

		for {
			if err := s.decCodecContext.ReceiveFrame(s.decFrame); err != nil {
				if errors.Is(err, astiav.ErrEof) || errors.Is(err, astiav.ErrEagain) {
					break
				}
				pw.CloseWithError(fmt.Errorf("receiving frame failed: %w", err))
				return
			}

			if s.decFrame.Pts() == astiav.NoPtsValue {
				s.decFrame.SetPts(s.nextFramePTS)
			}

			if s.decLastPTS != nil && *s.decLastPTS >= s.decFrame.Pts() {
				s.decFrame.SetPts(*s.decLastPTS + 1)
			}
			pts := s.decFrame.Pts()
			s.decLastPTS = &pts
			s.nextFramePTS = pts + 1

			if err := filterEncodeWriteFrame(s.decFrame, s, outputFormatContext); err != nil {
				pw.CloseWithError(fmt.Errorf("filter/encode failed: %w", err))
				return
			}
			s.decFrame.Unref()
		}
	}

cleanup:
	for _, s := range streams {
		if err := filterEncodeWriteFrame(nil, s, outputFormatContext); err != nil {
			pw.CloseWithError(err)
			return
		}
		if err := encodeWriteFrame(nil, s, outputFormatContext); err != nil {
			pw.CloseWithError(err)
			return
		}
	}

	if err := outputFormatContext.WriteTrailer(); err != nil {
		pw.CloseWithError(err)
		return
	}
}

func initTranscodeFilters(streams map[int]*transcodeStream, c *astikit.Closer) (err error) {
	for _, s := range streams {
		if s.filterGraph = astiav.AllocFilterGraph(); s.filterGraph == nil {
			return errors.New("graph is nil")
		}
		c.Add(s.filterGraph.Free)

		outputs := astiav.AllocFilterInOut()
		if outputs == nil {
			return errors.New("outputs is nil")
		}
		c.Add(outputs.Free)

		inputs := astiav.AllocFilterInOut()
		if inputs == nil {
			return errors.New("inputs is nil")
		}
		c.Add(inputs.Free)

		buffersrcContextParameters := astiav.AllocBuffersrcFilterContextParameters()
		defer buffersrcContextParameters.Free()

		var buffersrc, buffersink *astiav.Filter
		var content string
		switch s.decCodecContext.MediaType() {
		case astiav.MediaTypeAudio:
			buffersrc = astiav.FindFilterByName("abuffer")
			buffersrcContextParameters.SetChannelLayout(s.decCodecContext.ChannelLayout())
			buffersrcContextParameters.SetSampleFormat(s.decCodecContext.SampleFormat())
			buffersrcContextParameters.SetSampleRate(s.decCodecContext.SampleRate())
			buffersrcContextParameters.SetTimeBase(s.decCodecContext.TimeBase())
			buffersink = astiav.FindFilterByName("abuffersink")
			content = fmt.Sprintf("aformat=sample_fmts=%s:sample_rates=%d:channel_layouts=%s",
				s.encCodecContext.SampleFormat().Name(),
				s.encCodecContext.SampleRate(),
				s.encCodecContext.ChannelLayout().String())
			if s.encCodecContext.FrameSize() > 0 {
				content += fmt.Sprintf(",asetnsamples=n=%d", s.encCodecContext.FrameSize())
			}
		default:
			buffersrc = astiav.FindFilterByName("buffer")
			buffersrcContextParameters.SetHeight(s.decCodecContext.Height())
			buffersrcContextParameters.SetPixelFormat(s.decCodecContext.PixelFormat())
			buffersrcContextParameters.SetSampleAspectRatio(s.decCodecContext.SampleAspectRatio())
			buffersrcContextParameters.SetTimeBase(s.inputStream.TimeBase())
			buffersrcContextParameters.SetWidth(s.decCodecContext.Width())
			buffersink = astiav.FindFilterByName("buffersink")
			content = fmt.Sprintf("format=pix_fmts=%s", s.encCodecContext.PixelFormat().Name())
		}

		if s.buffersrcContext, err = s.filterGraph.NewBuffersrcFilterContext(buffersrc, "in"); err != nil {
			return fmt.Errorf("creating buffersrc failed: %w", err)
		}
		if s.buffersinkContext, err = s.filterGraph.NewBuffersinkFilterContext(buffersink, "out"); err != nil {
			return fmt.Errorf("creating buffersink failed: %w", err)
		}

		if err = s.buffersrcContext.SetParameters(buffersrcContextParameters); err != nil {
			return fmt.Errorf("setting buffersrc params failed: %w", err)
		}
		if err = s.buffersrcContext.Initialize(nil); err != nil {
			return fmt.Errorf("initializing buffersrc failed: %w", err)
		}

		outputs.SetName("in")
		outputs.SetFilterContext(s.buffersrcContext.FilterContext())
		outputs.SetPadIdx(0)
		outputs.SetNext(nil)

		inputs.SetName("out")
		inputs.SetFilterContext(s.buffersinkContext.FilterContext())
		inputs.SetPadIdx(0)
		inputs.SetNext(nil)

		if err = s.filterGraph.Parse(content, inputs, outputs); err != nil {
			return fmt.Errorf("parsing filter failed: %w", err)
		}
		if err = s.filterGraph.Configure(); err != nil {
			return fmt.Errorf("configuring filter failed: %w", err)
		}

		s.filterFrame = astiav.AllocFrame()
		c.Add(s.filterFrame.Free)

		s.encPkt = astiav.AllocPacket()
		c.Add(s.encPkt.Free)
	}
	return nil
}

func filterEncodeWriteFrame(f *astiav.Frame, s *transcodeStream, outputFormatContext *astiav.FormatContext) error {
	if err := s.buffersrcContext.AddFrame(f, astiav.NewBuffersrcFlags(astiav.BuffersrcFlagKeepRef)); err != nil {
		return fmt.Errorf("adding frame failed: %w", err)
	}
	for {
		if err := s.buffersinkContext.GetFrame(s.filterFrame, astiav.NewBuffersinkFlags()); err != nil {
			if errors.Is(err, astiav.ErrEof) || errors.Is(err, astiav.ErrEagain) {
				return nil
			}
			return fmt.Errorf("getting frame failed: %w", err)
		}
		frameTimeSeconds := timestampSeconds(s.filterFrame.Pts(), s.buffersinkContext.TimeBase())
		if frameTimeSeconds < s.segmentStartSeconds {
			s.filterFrame.Unref()
			continue
		}
		if s.segmentEndSeconds > 0 && frameTimeSeconds >= s.segmentEndSeconds {
			s.filterFrame.Unref()
			return nil
		}
		s.filterFrame.SetPictureType(astiav.PictureTypeNone)

		if err := encodeWriteFrame(s.filterFrame, s, outputFormatContext); err != nil {
			s.filterFrame.Unref()
			return err
		}
		s.filterFrame.Unref()
	}
}

func encodeWriteFrame(f *astiav.Frame, s *transcodeStream, outputFormatContext *astiav.FormatContext) error {
	if f != nil {
		if f.Pts() == astiav.NoPtsValue {
			f.SetPts(s.nextFramePTS)
		}
		pts := astiav.RescaleQ(f.Pts(), s.buffersinkContext.TimeBase(), s.encCodecContext.TimeBase())
		if s.segmentStartPTS < 0 {
			s.segmentStartPTS = pts
		}
		if pts < s.segmentStartPTS {
			pts = s.segmentStartPTS
		}
		pts -= s.segmentStartPTS
		if s.encLastPTS != nil && pts <= *s.encLastPTS {
			pts = *s.encLastPTS + 1
		}
		f.SetPts(pts)
		savedPTS := pts
		s.encLastPTS = &savedPTS
	}
	if err := s.encCodecContext.SendFrame(f); err != nil {
		return fmt.Errorf("sending frame failed: %w", err)
	}
	for {
		if err := s.encCodecContext.ReceivePacket(s.encPkt); err != nil {
			if errors.Is(err, astiav.ErrEof) || errors.Is(err, astiav.ErrEagain) {
				return nil
			}
			return fmt.Errorf("receiving packet failed: %w", err)
		}

		s.encPkt.SetStreamIndex(s.outputStream.Index())
		s.encPkt.RescaleTs(s.encCodecContext.TimeBase(), s.outputStream.TimeBase())
		if s.encPkt.Pts() == astiav.NoPtsValue {
			s.encPkt.SetPts(s.nextPacketDTS)
		}
		if s.encPkt.Dts() == astiav.NoPtsValue {
			s.encPkt.SetDts(s.encPkt.Pts())
		}
		if s.encPkt.Dts() > s.encPkt.Pts() {
			s.encPkt.SetPts(s.encPkt.Dts())
		}
		if s.encPkt.Dts() < s.nextPacketDTS {
			s.encPkt.SetDts(s.nextPacketDTS)
			if s.encPkt.Pts() < s.encPkt.Dts() {
				s.encPkt.SetPts(s.encPkt.Dts())
			}
		}
		s.nextPacketDTS = s.encPkt.Dts() + max64(1, s.encPkt.Duration())

		if err := outputFormatContext.WriteInterleavedFrame(s.encPkt); err != nil {
			s.encPkt.Unref()
			return fmt.Errorf("writing frame failed: %w", err)
		}
		s.encPkt.Unref()
	}
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
