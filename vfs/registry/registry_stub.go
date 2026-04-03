//go:build !windows

// Package registry exposes the Windows Registry as a vfs.FileSystem.
// On non-Windows platforms this stub is compiled instead; New() returns
// an error so callers can gracefully skip registry mounting.
package registry

import (
	"context"
	"errors"
	"io/fs"

	"wails-app/vfs"
)

// ErrNotSupported is returned by New() on non-Windows platforms.
var ErrNotSupported = errors.New("registry: Windows Registry is only available on Windows")

// FS is a placeholder type so the package compiles on all platforms.
type FS struct{}

// New always returns ErrNotSupported on non-Windows platforms.
func New() (*FS, error) {
	return nil, ErrNotSupported
}

// Compile-time interface check — FS must still satisfy vfs.FileSystem so that
// code referencing the type compiles, even though New() always errors.
var _ vfs.FileSystem = (*FS)(nil)

func (f *FS) Open(_ context.Context, _ string) (vfs.File, error)     { return nil, ErrNotSupported }
func (f *FS) Stat(_ context.Context, _ string) (vfs.FileInfo, error) { return nil, ErrNotSupported }
func (f *FS) ReadDir(_ context.Context, _ string) ([]vfs.FileInfo, error) {
	return nil, ErrNotSupported
}
func (f *FS) Exists(_ context.Context, _ string) bool                { return false }
func (f *FS) Create(_ context.Context, _ string) (vfs.File, error)   { return nil, ErrNotSupported }
func (f *FS) Mkdir(_ context.Context, _ string, _ fs.FileMode) error { return ErrNotSupported }
func (f *FS) Remove(_ context.Context, _ string) error               { return ErrNotSupported }
func (f *FS) Rename(_ context.Context, _, _ string) error            { return ErrNotSupported }
func (f *FS) Join(elem ...string) string                             { return "" }
func (f *FS) Split(_ string) (string, string)                        { return "", "" }
func (f *FS) IsAbs(_ string) bool                                    { return false }
func (f *FS) RealPath(_ string) (string, error)                      { return "", ErrNotSupported }
