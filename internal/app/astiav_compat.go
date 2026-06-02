//go:build cgo && (windows || darwin || linux)

package app

import "github.com/asticode/go-astiav"

// Compatibility constants that mirror FFmpeg C macros.
//
// Some FFmpeg installations / header versions don't expose the exact symbols
// that go-astiav binds to. These values are plain numeric flags, so duplicating
// them locally keeps the CGO-tagged files compilable regardless of the FFmpeg
// version in the environment.

const (
	// AV_DISPOSITION_ATTACHED_PIC = (1 << 10)
	astiavDispositionFlagAttachedPic = astiav.DispositionFlag(1 << 10)

	// AVSEEK_FLAG_BACKWARD = (1 << 1)
	astiavSeekFlagBackward = astiav.SeekFlag(1 << 1)

	// AV_PKT_FLAG_KEY = 0x1
	astiavPacketFlagKey = astiav.PacketFlag(0x1)

	// SWS_POINT  = 0x10
	// SWS_BILINEAR = 0x2
	astiavSoftwareScaleContextFlagPoint    = astiav.SoftwareScaleContextFlag(0x10)
	astiavSoftwareScaleContextFlagBilinear = astiav.SoftwareScaleContextFlag(0x2)

	// AVFMT_GLOBALHEADER = 0x40
	astiavIOFormatFlagGlobalheader = astiav.IOFormatFlag(0x40)

	// AV_CODEC_FLAG_GLOBAL_HEADER = (1 << 22)
	astiavCodecContextFlagGlobalHeader = astiav.CodecContextFlag(1 << 22)

	// AV_NOPTS_VALUE = 0x8000000000000000 (as int64)
	astiavNoPtsValue = int64(-9223372036854775808)

	// AV_TIME_BASE = 1 000 000
	astiavTimeBase = int(1000000)

	// AVERROR(EAGAIN) = -11
	astiavErrEagain = astiav.Error(-11)

	// AVERROR_EOF = -541478725
	astiavErrEof = astiav.Error(-541478725)
)

func findAstiavEncoderByNames(names ...string) *astiav.Codec {
	for _, name := range names {
		if encoder := astiav.FindEncoderByName(name); encoder != nil {
			return encoder
		}
	}
	return nil
}
