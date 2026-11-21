package archive

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"

	bzip2writer "github.com/dsnet/compress/bzip2"
	"github.com/mholt/archiver/v3"
)

func rewriteZip(fsPath string, entries []string, op Operation) (removed, kept int, err error) {
	r, err := zip.OpenReader(fsPath)
	if err != nil {
		return 0, 0, err
	}
	defer r.Close()

	op.SetTotal(int64(len(r.File)), 0)

	tmp, err := os.CreateTemp(filepath.Dir(fsPath), ".archive-rewrite-*.zip")
	if err != nil {
		return 0, 0, err
	}
	tmpPath := tmp.Name()
	defer func() {
		tmp.Close()
		if err != nil {
			os.Remove(tmpPath)
		}
	}()

	w := zip.NewWriter(tmp)
	defer w.Close()

	var processed int64
	for _, f := range r.File {
		if op.IsCancelled() {
			return removed, kept, fmt.Errorf("cancelled")
		}

		innerPath := normalizeArchiveEntryPath(f.Name)
		if matchesEntries(innerPath, entries) {
			removed++
			processed++
			op.UpdateProgress(processed, 0, f.Name, 0)
			continue
		}

		rc, openErr := f.Open()
		if openErr != nil {
			return removed, kept, fmt.Errorf("open %q: %w", f.Name, openErr)
		}

		hdr, _ := zip.FileInfoHeader(f.FileInfo())
		hdr.Name = f.Name
		hdr.Method = f.Method
		fw, writeErr := w.CreateHeader(hdr)
		if writeErr != nil {
			rc.Close()
			return removed, kept, writeErr
		}

		n, copyErr := io.Copy(fw, rc)
		rc.Close()
		if copyErr != nil {
			return removed, kept, fmt.Errorf("copy %q: %w", f.Name, copyErr)
		}

		kept++
		processed++
		op.UpdateProgress(processed, n, f.Name, n)
	}

	if err = w.Close(); err != nil {
		return removed, kept, err
	}
	tmp.Close()
	// Release the source archive before replacing it: Windows refuses to
	// rename over a file that still has an open handle.
	r.Close()
	return removed, kept, os.Rename(tmpPath, fsPath)
}

func rewriteTar(fsPath string, entries []string, compression string, op Operation) (removed, kept int, err error) {
	srcFile, err := os.Open(fsPath)
	if err != nil {
		return 0, 0, err
	}
	defer srcFile.Close()

	var tarReader *tar.Reader
	switch compression {
	case "gz":
		gr, gzErr := gzip.NewReader(srcFile)
		if gzErr != nil {
			return 0, 0, gzErr
		}
		defer gr.Close()
		tarReader = tar.NewReader(gr)
	case "bz2":
		tarReader = tar.NewReader(bzip2.NewReader(srcFile))
	default:
		tarReader = tar.NewReader(srcFile)
	}

	if compression == "xz" {
		return rewriteTarXzViaArchiver(fsPath, entries, op)
	}

	tmp, err := os.CreateTemp(filepath.Dir(fsPath), ".archive-rewrite-*")
	if err != nil {
		return 0, 0, err
	}
	tmpPath := tmp.Name()
	defer func() {
		tmp.Close()
		if err != nil {
			os.Remove(tmpPath)
		}
	}()

	var tarWriter *tar.Writer
	var gzWriter *gzip.Writer
	var bzWriter *bzip2writer.Writer
	switch compression {
	case "gz":
		gzWriter = gzip.NewWriter(tmp)
		tarWriter = tar.NewWriter(gzWriter)
	case "bz2":
		// Go's stdlib bzip2 is read-only; dsnet/compress provides the writer
		// so a rewritten .tar.bz2 stays a bzip2 container.
		bzWriter, err = bzip2writer.NewWriter(tmp, nil)
		if err != nil {
			return removed, kept, err
		}
		tarWriter = tar.NewWriter(bzWriter)
	default:
		tarWriter = tar.NewWriter(tmp)
	}

	var processed int64
	for {
		if op.IsCancelled() {
			tarWriter.Close()
			if gzWriter != nil {
				gzWriter.Close()
			}
			if bzWriter != nil {
				bzWriter.Close()
			}
			return removed, kept, fmt.Errorf("cancelled")
		}

		hdr, tarErr := tarReader.Next()
		if tarErr == io.EOF {
			break
		}
		if tarErr != nil {
			return removed, kept, tarErr
		}

		innerPath := normalizeArchiveEntryPath(hdr.Name)
		if matchesEntries(innerPath, entries) {
			removed++
			processed++
			op.UpdateProgress(processed, 0, hdr.Name, 0)
			continue
		}

		if writeErr := tarWriter.WriteHeader(hdr); writeErr != nil {
			return removed, kept, writeErr
		}
		n, copyErr := io.Copy(tarWriter, tarReader)
		if copyErr != nil {
			return removed, kept, copyErr
		}
		kept++
		processed++
		op.UpdateProgress(processed, n, hdr.Name, n)
	}

	if e := tarWriter.Close(); e != nil {
		return removed, kept, e
	}
	if gzWriter != nil {
		if e := gzWriter.Close(); e != nil {
			return removed, kept, e
		}
	}
	if bzWriter != nil {
		if e := bzWriter.Close(); e != nil {
			return removed, kept, e
		}
	}
	tmp.Close()
	// Release the source archive before replacing it: Windows refuses to
	// rename over a file that still has an open handle.
	srcFile.Close()
	return removed, kept, os.Rename(tmpPath, fsPath)
}

func rewriteTarXzViaArchiver(fsPath string, entries []string, op Operation) (removed, kept int, err error) {
	type keptEntry struct {
		hdr  *tar.Header
		data []byte
	}
	var keptEntries []keptEntry

	walkErr := archiver.Walk(fsPath, func(f archiver.File) error {
		if op.IsCancelled() {
			return fmt.Errorf("cancelled")
		}
		hdr, ok := f.Header.(*tar.Header)
		if !ok {
			return nil
		}
		innerPath := normalizeArchiveEntryPath(hdr.Name)
		if matchesEntries(innerPath, entries) {
			removed++
			return nil
		}
		data, rerr := io.ReadAll(f)
		if rerr != nil {
			return rerr
		}
		keptEntries = append(keptEntries, keptEntry{hdr: hdr, data: data})
		kept++
		return nil
	})
	if walkErr != nil {
		return removed, kept, walkErr
	}

	tmp, err := os.CreateTemp(filepath.Dir(fsPath), ".archive-rewrite-*.tar.xz")
	if err != nil {
		return removed, kept, err
	}
	tmpPath := tmp.Name()
	defer func() {
		tmp.Close()
		if err != nil {
			os.Remove(tmpPath)
		}
	}()

	txz := archiver.NewTarXz()
	if e := txz.Create(tmp); e != nil {
		return removed, kept, fmt.Errorf("create tar.xz: %w", e)
	}

	var processed int64
	for _, ke := range keptEntries {
		if err = txz.Write(archiver.File{
			FileInfo: archiver.FileInfo{
				FileInfo:   ke.hdr.FileInfo(),
				CustomName: ke.hdr.Name,
			},
			ReadCloser: io.NopCloser(bytes.NewReader(ke.data)),
		}); err != nil {
			txz.Close()
			return removed, kept, fmt.Errorf("write %q: %w", ke.hdr.Name, err)
		}
		processed++
		op.UpdateProgress(processed, int64(len(ke.data)), ke.hdr.Name, int64(len(ke.data)))
	}

	if err = txz.Close(); err != nil {
		return removed, kept, err
	}
	tmp.Close()
	return removed, kept, os.Rename(tmpPath, fsPath)
}
