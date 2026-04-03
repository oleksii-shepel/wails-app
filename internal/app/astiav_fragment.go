package app

import (
"bytes"
"context"
"fmt"

"github.com/asticode/go-astiav"
)

func buildSegmentedVideoFragment(ctx context.Context, nativePath string, startSeconds float64, durationSeconds float64) ([]byte, []byte, error) {
inputFormatContext := astiav.AllocFormatContext()
if inputFormatContext == nil {
return nil, nil, fmt.Errorf("failed allocating input format context")
}
defer inputFormatContext.Free()

if err := inputFormatContext.OpenInput(nativePath, nil, nil); err != nil {
return nil, nil, fmt.Errorf("failed opening input: %w", err)
}
defer inputFormatContext.CloseInput()

if err := inputFormatContext.FindStreamInfo(nil); err != nil {
return nil, nil, fmt.Errorf("failed finding stream info: %w", err)
}

if startSeconds > 0 {
seekTarget := int64(startSeconds * float64(astiav.TimeBase))
if err := inputFormatContext.SeekFrame(-1, seekTarget, astiav.NewSeekFlags(astiav.SeekFlagBackward)); err != nil {
// Non-fatal seek error
}
}

outputFormatContext, err := astiav.AllocOutputFormatContext(nil, "mp4", "")
if err != nil {
return nil, nil, fmt.Errorf("failed allocating output format context: %w", err)
}
defer outputFormatContext.Free()

var buf bytes.Buffer
ioContext, err := astiav.AllocIOContext(
4096,
true,
nil,
nil,
func(b []byte) (int, error) {
return buf.Write(b)
},
)
if err != nil {
return nil, nil, fmt.Errorf("failed allocating custom IO context: %w", err)
}
defer ioContext.Free()

outputFormatContext.SetPb(ioContext)

streamMapping := make(map[int]int)
outputStreamIdx := 0

streamsArr := inputFormatContext.Streams()
for i := 0; i < inputFormatContext.NbStreams(); i++ {
inStream := streamsArr[i]
codecParams := inStream.CodecParameters()
if codecParams.MediaType() != astiav.MediaTypeVideo && codecParams.MediaType() != astiav.MediaTypeAudio {
streamMapping[i] = -1
continue
}

outStream := outputFormatContext.NewStream(nil)
if outStream == nil {
return nil, nil, fmt.Errorf("failed allocating output stream")
}

if err := codecParams.Copy(outStream.CodecParameters()); err != nil {
return nil, nil, fmt.Errorf("failed copying codec parameters: %w", err)
}
outStream.CodecParameters().SetCodecTag(0)

streamMapping[i] = outputStreamIdx
outputStreamIdx++
}

options := astiav.NewDictionary()
defer options.Free()
options.Set("movflags", "frag_keyframe+empty_moov+default_base_moof", 0)

if err := outputFormatContext.WriteHeader(options); err != nil {
return nil, nil, fmt.Errorf("failed writing header: %w", err)
}

pkt := astiav.AllocPacket()
if pkt == nil {
return nil, nil, fmt.Errorf("failed allocating packet")
}
defer pkt.Free()

var startPts int64 = -1

for {
if err := inputFormatContext.ReadFrame(pkt); err != nil {
break
}

inStreamIdx := pkt.StreamIndex()
outStreamIdx, ok := streamMapping[inStreamIdx]
if !ok || outStreamIdx < 0 {
pkt.Unref()
continue
}

inStream := inputFormatContext.Streams()[inStreamIdx]
outStream := outputFormatContext.Streams()[outStreamIdx]

if inStream.CodecParameters().MediaType() == astiav.MediaTypeVideo {
if startPts == -1 {
startPts = pkt.Pts()
}
if durationSeconds > 0 && startPts != -1 {
timeDiff := float64(pkt.Pts()-startPts) * float64(inStream.TimeBase().Num()) / float64(inStream.TimeBase().Den())
if timeDiff > durationSeconds {
pkt.Unref()
break
}
}
}

pkt.SetStreamIndex(outStreamIdx)
pkt.RescaleTs(inStream.TimeBase(), outStream.TimeBase())
pkt.SetPos(-1)

if err := outputFormatContext.WriteInterleavedFrame(pkt); err != nil {
pkt.Unref()
return nil, nil, fmt.Errorf("failed writing frame: %w", err)
}
pkt.Unref()
}

if err := outputFormatContext.WriteTrailer(); err != nil {
return nil, nil, fmt.Errorf("failed writing trailer: %w", err)
}

return splitFragmentedMP4(buf.Bytes())
}
