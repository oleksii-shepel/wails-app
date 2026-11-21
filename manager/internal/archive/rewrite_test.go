package archive

import (
	"archive/tar"
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
)

type rewriteStubOp struct{}

func (rewriteStubOp) ID() string                                 { return "test" }
func (rewriteStubOp) Complete()                                  {}
func (rewriteStubOp) SetError(error)                             {}
func (rewriteStubOp) SetTotal(int64, int64)                      {}
func (rewriteStubOp) UpdateProgress(int64, int64, string, int64) {}
func (rewriteStubOp) IsCancelled() bool                          { return false }

type rewriteEntry struct {
	name string
	body string
}

func writeTarEntries(t *testing.T, tw *tar.Writer, entries []rewriteEntry) {
	t.Helper()
	for _, e := range entries {
		if err := tw.WriteHeader(&tar.Header{
			Name: e.name,
			Mode: 0o644,
			Size: int64(len(e.body)),
		}); err != nil {
			t.Fatalf("write header %q: %v", e.name, err)
		}
		if _, err := io.Copy(tw, bytes.NewReader([]byte(e.body))); err != nil {
			t.Fatalf("write body %q: %v", e.name, err)
		}
	}
}

func createCompressedTar(t *testing.T, path, compression string, entries []rewriteEntry) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	defer f.Close()

	switch compression {
	case "gz":
		gz := gzip.NewWriter(f)
		tw := tar.NewWriter(gz)
		writeTarEntries(t, tw, entries)
		if err := tw.Close(); err != nil {
			t.Fatalf("close tar: %v", err)
		}
		if err := gz.Close(); err != nil {
			t.Fatalf("close gzip: %v", err)
		}
	case "bz2":
		t.Fatalf("bz2 rewrite covered by TestRewriteTarBz2KeepsBzip2Container")
	case "":
		tw := tar.NewWriter(f)
		writeTarEntries(t, tw, entries)
		if err := tw.Close(); err != nil {
			t.Fatalf("close tar: %v", err)
		}
	default:
		t.Fatalf("unknown compression %q", compression)
	}
}

func readTarEntries(t *testing.T, r io.Reader) []rewriteEntry {
	t.Helper()
	tr := tar.NewReader(r)
	var got []rewriteEntry
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("read tar: %v", err)
		}
		body, err := io.ReadAll(tr)
		if err != nil {
			t.Fatalf("read body %q: %v", hdr.Name, err)
		}
		got = append(got, rewriteEntry{name: hdr.Name, body: string(body)})
	}
	return got
}

func readCompressedTarEntries(t *testing.T, path, compression string) []rewriteEntry {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()

	switch compression {
	case "gz":
		gr, err := gzip.NewReader(f)
		if err != nil {
			t.Fatalf("gzip reader: %v", err)
		}
		defer gr.Close()
		return readTarEntries(t, gr)
	case "bz2":
		return readTarEntries(t, bzip2.NewReader(f))
	case "":
		return readTarEntries(t, f)
	default:
		t.Fatalf("unknown compression %q", compression)
		return nil
	}
}

func TestRewriteTarRemovesEntriesAndStaysReadable(t *testing.T) {
	entries := []rewriteEntry{
		{name: "a.txt", body: "alpha"},
		{name: "remove-me.txt", body: "gone"},
		{name: "sub/c.txt", body: "gamma"},
	}
	for _, compression := range []string{"", "gz"} {
		t.Run(compression, func(t *testing.T) {
			dir := t.TempDir()
			archivePath := filepath.Join(dir, "sample.tar"+compressionSuffix(compression))
			createCompressedTar(t, archivePath, compression, entries)

			removed, kept, err := rewriteTar(archivePath, []string{"remove-me.txt"}, compression, rewriteStubOp{})
			if err != nil {
				t.Fatalf("rewriteTar: %v", err)
			}
			if removed != 1 || kept != 2 {
				t.Fatalf("removed=%d kept=%d, want 1 and 2", removed, kept)
			}

			got := readCompressedTarEntries(t, archivePath, compression)
			if len(got) != 2 {
				t.Fatalf("got %d entries, want 2: %+v", len(got), got)
			}
			if got[0].name != "a.txt" || got[0].body != "alpha" {
				t.Fatalf("entry[0] = %+v, want a.txt/alpha", got[0])
			}
			if got[1].name != "sub/c.txt" || got[1].body != "gamma" {
				t.Fatalf("entry[1] = %+v, want sub/c.txt/gamma", got[1])
			}
		})
	}
}

func compressionSuffix(compression string) string {
	switch compression {
	case "gz":
		return ".gz"
	case "bz2":
		return ".bz2"
	default:
		return ""
	}
}
