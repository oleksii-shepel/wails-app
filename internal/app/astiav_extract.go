package app

import (
"bytes"
"context"
"errors"
"fmt"
"time"

"github.com/asticode/go-astiav"
)

func extractVideoFrame(ctx context.Context, inputPath string, targetFormat string, frameSeconds float64) ([]byte, error) {
fmtCtx := astiav.AllocFormatContext()
if fmtCtx == nil {
return nil, errors.New("failed allocating format context")
}
defer fmtCtx.Free()

if err := fmtCtx.OpenInput(inputPath, nil, nil); err != nil {
return nil, fmt.Errorf("failed opening input: %w", err)
}
defer fmtCtx.CloseInput()

if err := fmtCtx.FindStreamInfo(nil); err != nil {
return nil, fmt.Errorf("failed finding stream info: %w", err)
}

var videoStream *astiav.Stream
var streamIdx int = -1
for i := 0; i < fmtCtx.NbStreams(); i++ {
s := fmtCtx.Streams()[i]
if s.CodecParameters().MediaType() == astiav.MediaTypeVideo && !s.DispositionFlags().Has(astiav.DispositionFlagAttachedPic) {
videoStream = s
streamIdx = i
break
}
}
if videoStream == nil {
return nil, errors.New("no video stream found")
}

if frameSeconds > 0 {
seekTarget := int64(frameSeconds * float64(astiav.TimeBase))
_ = fmtCtx.SeekFrame(-1, seekTarget, astiav.NewSeekFlags(astiav.SeekFlagBackward))
}

decoder := astiav.FindDecoder(videoStream.CodecParameters().CodecID())
if decoder == nil {
return nil, errors.New("video decoder not found")
}

decCtx := astiav.AllocCodecContext(decoder)
if decCtx == nil {
return nil, errors.New("failed allocating decoder context")
}
defer decCtx.Free()

if err := videoStream.CodecParameters().ToCodecContext(decCtx); err != nil {
return nil, fmt.Errorf("failed copying codec params to decoder context: %w", err)
}

if err := decCtx.Open(decoder, nil); err != nil {
return nil, fmt.Errorf("failed opening decoder: %w", err)
}

pkt := astiav.AllocPacket()
if pkt == nil {
return nil, errors.New("failed allocating packet")
}
defer pkt.Free()

frame := astiav.AllocFrame()
if frame == nil {
return nil, errors.New("failed allocating frame")
}
defer frame.Free()

var decodedFrame bool
timeout := time.After(5 * time.Second)

decodeLoop:
for {
select {
case <-ctx.Done():
return nil, ctx.Err()
case <-timeout:
return nil, errors.New("timeout reading frame")
default:
}

if err := fmtCtx.ReadFrame(pkt); err != nil {
if errors.Is(err, astiav.ErrEof) {
break
}
return nil, fmt.Errorf("reading frame: %w", err)
}

if pkt.StreamIndex() != streamIdx {
pkt.Unref()
continue
}

if err := decCtx.SendPacket(pkt); err != nil {
pkt.Unref()
return nil, fmt.Errorf("sending packet to decoder: %w", err)
}
pkt.Unref()

for {
err := decCtx.ReceiveFrame(frame)
if errors.Is(err, astiav.ErrEagain) || errors.Is(err, astiav.ErrEof) {
break
}
if err != nil {
return nil, fmt.Errorf("receiving frame from decoder: %w", err)
}
decodedFrame = true
break decodeLoop
}
}

if !decodedFrame {
return nil, errors.New("no frame decoded")
}

// Prepare encoder (PNG or MJPEG)
encCodecID := astiav.CodecIDPng
if targetFormat == "mjpeg" || targetFormat == "jpg" || targetFormat == "jpeg" {
encCodecID = astiav.CodecIDMjpeg
}

encoder := astiav.FindEncoder(encCodecID)
if encoder == nil {
return nil, errors.New("video encoder not found")
}

encCtx := astiav.AllocCodecContext(encoder)
if encCtx == nil {
return nil, errors.New("failed allocating encoder context")
}
defer encCtx.Free()

// Set encoder params
encCtx.SetHeight(decCtx.Height())
encCtx.SetWidth(decCtx.Width())
encCtx.SetTimeBase(astiav.NewRational(1, 25))

// Choose supported pixel format for encoder
p := []astiav.PixelFormat{}
if len(p) > 0 {
encCtx.SetPixelFormat(p[0])
} else {
encCtx.SetPixelFormat(astiav.PixelFormatRgba)
}

if err := encCtx.Open(encoder, nil); err != nil {
return nil, fmt.Errorf("failed opening encoder: %w", err)
}

// Scale/Convert frame if necessary
scaledFrame := frame
if frame.PixelFormat() != encCtx.PixelFormat() || frame.Width() != encCtx.Width() || frame.Height() != encCtx.Height() {
swsCtx, err := astiav.CreateSoftwareScaleContext(
frame.Width(), frame.Height(), frame.PixelFormat(),
encCtx.Width(), encCtx.Height(), encCtx.PixelFormat(),
astiav.SoftwareScaleContextFlags(astiav.SoftwareScaleContextFlagPoint),
)
if err != nil {
return nil, fmt.Errorf("failed creating sws context: %w", err)
}
defer swsCtx.Free()

scaledFrame = astiav.AllocFrame()
if scaledFrame == nil {
return nil, errors.New("failed allocating scaled frame")
}
defer scaledFrame.Free()

scaledFrame.SetWidth(encCtx.Width())
scaledFrame.SetHeight(encCtx.Height())
scaledFrame.SetPixelFormat(encCtx.PixelFormat())
if err := scaledFrame.AllocBuffer(1); err != nil {
return nil, fmt.Errorf("failed allocating buffer for scaled frame: %w", err)
}

if err := swsCtx.ScaleFrame(frame, scaledFrame); err != nil {
return nil, fmt.Errorf("failed scaling frame: %w", err)
}
}

if err := encCtx.SendFrame(scaledFrame); err != nil {
return nil, fmt.Errorf("sending frame to encoder: %w", err)
}
if err := encCtx.SendFrame(nil); err != nil && !errors.Is(err, astiav.ErrEof) {
return nil, fmt.Errorf("flushing encoder: %w", err)
}

var outBuffer bytes.Buffer
encPkt := astiav.AllocPacket()
if encPkt == nil {
return nil, errors.New("failed allocating encoder packet")
}
defer encPkt.Free()

for {
err := encCtx.ReceivePacket(encPkt)
if errors.Is(err, astiav.ErrEagain) || errors.Is(err, astiav.ErrEof) {
break
}
if err != nil {
return nil, fmt.Errorf("receiving encoder packet: %w", err)
}
outBuffer.Write(encPkt.Data())
encPkt.Unref()
}

if outBuffer.Len() == 0 {
return nil, errors.New("encoder produced no output")
}

return outBuffer.Bytes(), nil
}
