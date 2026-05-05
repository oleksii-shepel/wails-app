//go:build cgo && (windows || darwin || linux)

package app

import "github.com/asticode/go-astiav"

const (
	// FFmpeg defines AV_DISPOSITION_ATTACHED_PIC as (1 << 10) in avformat.h.
	// Using the literal here avoids depending on astiav's exported constant,
	// which can become an invalid typed constant with some FFmpeg header sets.
	astiavDispositionFlagAttachedPic = astiav.DispositionFlag(1 << 10)
)
