package cloudutil

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"net/http"

	"wails-app/vfs"
)

const downloadChunkSize = 4 * 1024 * 1024

// RangeLoader opens an HTTP response for the requested byte range. The caller
// owns the response body.
type RangeLoader func(context.Context, string) (*http.Response, error)

// ReadOnlyFile streams a cloud file in bounded ranges and reopens the stream
// at the next read after a seek.
type ReadOnlyFile struct {
	ctx       context.Context
	info      vfs.FileInfo
	load      RangeLoader
	body      io.ReadCloser
	offset    int64
	remaining int64
	closed    bool
}

func NewReadOnlyFile(ctx context.Context, info vfs.FileInfo, load RangeLoader) *ReadOnlyFile {
	return &ReadOnlyFile{ctx: ctx, info: info, load: load}
}

func (f *ReadOnlyFile) openRange() error {
	length := min(int64(downloadChunkSize), f.info.Size()-f.offset)
	response, err := f.load(f.ctx, fmt.Sprintf("bytes=%d-%d", f.offset, f.offset+length-1))
	if err != nil {
		return err
	}

	if response.StatusCode == http.StatusPartialContent {
		var start, end, size int64
		header := response.Header.Get("Content-Range")
		if _, err := fmt.Sscanf(header, "bytes %d-%d/%d", &start, &end, &size); err != nil ||
			start != f.offset || end < start || end-start >= length || size != f.info.Size() {
			response.Body.Close()
			return fmt.Errorf("cloud download: unexpected Content-Range %q", header)
		}
		length = end - start + 1
	} else if response.StatusCode == http.StatusOK {
		// Some endpoints ignore Range. Skip the prefix without allocating it
		// and reuse the response until the next seek or end of the file.
		if _, err := io.CopyN(io.Discard, response.Body, f.offset); err != nil {
			response.Body.Close()
			return err
		}
		length = f.info.Size() - f.offset
	} else {
		response.Body.Close()
		return fmt.Errorf("cloud download: unexpected HTTP status %d", response.StatusCode)
	}

	f.body = response.Body
	f.remaining = length
	return nil
}

func (f *ReadOnlyFile) Read(p []byte) (int, error) {
	if f.closed {
		return 0, fs.ErrClosed
	}
	if len(p) == 0 {
		return 0, nil
	}
	if f.offset >= f.info.Size() {
		return 0, io.EOF
	}
	if f.body == nil {
		if err := f.openRange(); err != nil {
			return 0, err
		}
	}
	n, err := f.body.Read(p[:min(int64(len(p)), f.remaining)])
	f.offset += int64(n)
	f.remaining -= int64(n)
	if f.remaining == 0 || err != nil {
		_ = f.closeBody()
	}
	if err == io.EOF {
		if f.remaining != 0 {
			err = io.ErrUnexpectedEOF
		} else if f.offset < f.info.Size() {
			err = nil
		}
	}
	return n, err
}

func (f *ReadOnlyFile) Seek(offset int64, whence int) (int64, error) {
	if f.closed {
		return 0, fs.ErrClosed
	}
	var base int64
	switch whence {
	case io.SeekStart:
	case io.SeekCurrent:
		base = f.offset
	case io.SeekEnd:
		base = f.info.Size()
	default:
		return 0, fmt.Errorf("cloud download: invalid seek whence %d", whence)
	}
	next := base + offset
	if next < 0 {
		return 0, fmt.Errorf("cloud download: invalid seek position")
	}
	if next != f.offset {
		_ = f.closeBody()
		f.offset = next
	}
	return next, nil
}

func (f *ReadOnlyFile) closeBody() error {
	if f.body == nil {
		return nil
	}
	err := f.body.Close()
	f.body = nil
	return err
}

func (f *ReadOnlyFile) Close() error {
	f.closed = true
	return f.closeBody()
}

func (f *ReadOnlyFile) Stat() (vfs.FileInfo, error) { return f.info, nil }
func (f *ReadOnlyFile) Write([]byte) (int, error)   { return 0, vfs.ErrReadOnly }
func (f *ReadOnlyFile) Sync() error                 { return vfs.ErrReadOnly }

var _ vfs.File = (*ReadOnlyFile)(nil)
