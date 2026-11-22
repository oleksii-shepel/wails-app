package cloudutil

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReadOnlyFileSeeksWithoutDownloadingPrefix(t *testing.T) {
	const size int64 = 5 << 30
	var requests []string
	file := NewReadOnlyFile(context.Background(), &Info{SizeValue: size}, func(_ context.Context, requested string) (*http.Response, error) {
		requests = append(requests, requested)
		return &http.Response{
			StatusCode: http.StatusPartialContent,
			Header:     http.Header{"Content-Range": {fmt.Sprintf("bytes %d-%d/%d", size-3, size-1, size)}},
			Body:       io.NopCloser(strings.NewReader("end")),
		}, nil
	})
	defer file.Close()
	if offset, err := file.Seek(-3, io.SeekEnd); err != nil || offset != size-3 || len(requests) != 0 {
		t.Fatalf("seek = (%d, %v), requests = %v", offset, err, requests)
	}
	data, err := io.ReadAll(file)
	if err != nil || string(data) != "end" || len(requests) != 1 || requests[0] != fmt.Sprintf("bytes=%d-%d", size-3, size-1) {
		t.Fatalf("read = (%q, %v), requests = %v", data, err, requests)
	}
}

func TestReadOnlyFileStreamsFullResponseWhenRangeIsIgnored(t *testing.T) {
	file := NewReadOnlyFile(context.Background(), &Info{SizeValue: 6}, func(_ context.Context, _ string) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("abcdef"))}, nil
	})
	defer file.Close()
	if _, err := file.Seek(3, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(file)
	if err != nil || string(data) != "def" {
		t.Fatalf("read = (%q, %v)", data, err)
	}
}

func TestReadOnlyFileRejectsResponseAtWrongOffset(t *testing.T) {
	file := NewReadOnlyFile(context.Background(), &Info{SizeValue: 6}, func(_ context.Context, _ string) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusPartialContent,
			Header:     http.Header{"Content-Range": {"bytes 0-2/6"}},
			Body:       io.NopCloser(strings.NewReader("abc")),
		}, nil
	})
	defer file.Close()
	if _, err := file.Seek(3, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	if n, err := file.Read(make([]byte, 3)); n != 0 || err == nil {
		t.Fatalf("read = (%d, %v), want a range error", n, err)
	}
}

func TestReadOnlyFileServesMediaRange(t *testing.T) {
	const size int64 = 5 << 30
	var requests []string
	file := NewReadOnlyFile(context.Background(), &Info{SizeValue: size}, func(_ context.Context, requested string) (*http.Response, error) {
		requests = append(requests, requested)
		return &http.Response{
			StatusCode: http.StatusPartialContent,
			Header:     http.Header{"Content-Range": {fmt.Sprintf("bytes %d-%d/%d", size-3, size-1, size)}},
			Body:       io.NopCloser(strings.NewReader("end")),
		}, nil
	})
	defer file.Close()
	req := httptest.NewRequest(http.MethodGet, "/video.mp4", nil)
	req.Header.Set("Range", "bytes=-3")
	recorder := httptest.NewRecorder()
	recorder.Header().Set("Content-Type", "video/mp4")
	http.ServeContent(recorder, req, "video.mp4", file.info.ModTime(), file)
	if recorder.Code != http.StatusPartialContent || recorder.Body.String() != "end" || len(requests) != 1 {
		t.Fatalf("preview = (%d, %q), upstream requests = %v", recorder.Code, recorder.Body.String(), requests)
	}
}
