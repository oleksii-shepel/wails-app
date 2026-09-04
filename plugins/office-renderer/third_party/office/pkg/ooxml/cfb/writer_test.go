package cfb_test

import (
	"bytes"
	"crypto/rand"
	"sort"
	"strings"
	"testing"

	"office/pkg/ooxml/cfb"
)

// roundTrip writes the given streams and verifies the resulting bytes parse
// and yield the same contents through cfb.Open.
func roundTrip(t *testing.T, streams map[string][]byte) {
	t.Helper()
	w := cfb.NewWriter()
	names := make([]string, 0, len(streams))
	for name := range streams {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err := w.AddStream(name, streams[name]); err != nil {
			t.Fatalf("AddStream(%q): %v", name, err)
		}
	}
	raw, err := w.Bytes()
	if err != nil {
		t.Fatalf("Bytes: %v", err)
	}

	f, err := cfb.OpenBytes(raw)
	if err != nil {
		t.Fatalf("OpenBytes: %v", err)
	}

	gotNames := append([]string(nil), f.StreamNames()...)
	sort.Strings(gotNames)
	if !equalStrings(gotNames, names) {
		t.Fatalf("StreamNames = %v, want %v", gotNames, names)
	}
	for _, name := range names {
		got, ok := f.Stream(name)
		if !ok {
			t.Fatalf("Stream(%q): not found", name)
		}
		want := streams[name]
		if !bytes.Equal(got, want) {
			t.Fatalf("Stream(%q) = %d bytes, want %d bytes (mismatch)", name, len(got), len(want))
		}
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestWriter_RoundTripSingleSmallStream(t *testing.T) {
	roundTrip(t, map[string][]byte{
		"Hello": []byte("hello world"),
	})
}

func TestWriter_RoundTripMultipleMiniStreams(t *testing.T) {
	roundTrip(t, map[string][]byte{
		"alpha":   []byte("a-payload"),
		"beta":    []byte("b-payload-longer"),
		"gamma":   bytes.Repeat([]byte{0x42}, 200),
		"epsilon": []byte("eps"),
	})
}

func TestWriter_RoundTripLargeStream(t *testing.T) {
	// 4096 bytes is the cutoff; use a payload comfortably above it so the
	// stream goes through the main FAT.
	payload := bytes.Repeat([]byte("LARGE-DATA-"), 1000) // 11 000 bytes
	roundTrip(t, map[string][]byte{
		"BigStream": payload,
	})
}

func TestWriter_RoundTripMixedMiniAndLarge(t *testing.T) {
	big := make([]byte, 8200)
	if _, err := rand.Read(big); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}
	roundTrip(t, map[string][]byte{
		"EncryptionInfo":   bytes.Repeat([]byte{0xAB}, 1024), // mini
		"EncryptedPackage": big,                              // main FAT
		"tag":              []byte("x"),                      // mini
	})
}

func TestWriter_RoundTripManyStreamsForcesMultiFAT(t *testing.T) {
	// One large 70 KB stream → 137 sectors + overhead, more than a single FAT
	// sector (128 entries) can index.
	payload := make([]byte, 70*1024)
	for i := range payload {
		payload[i] = byte(i)
	}
	roundTrip(t, map[string][]byte{
		"WordDocument": payload,
		"Summary":      []byte("summary metadata"),
	})
}

func TestWriter_RoundTripEmptyStream(t *testing.T) {
	w := cfb.NewWriter()
	if err := w.AddStream("Empty", nil); err != nil {
		t.Fatalf("AddStream: %v", err)
	}
	if err := w.AddStream("Real", []byte("payload")); err != nil {
		t.Fatalf("AddStream: %v", err)
	}
	raw, err := w.Bytes()
	if err != nil {
		t.Fatalf("Bytes: %v", err)
	}
	f, err := cfb.OpenBytes(raw)
	if err != nil {
		t.Fatalf("OpenBytes: %v", err)
	}
	got, ok := f.Stream("Empty")
	if !ok {
		t.Fatal("Empty stream not found")
	}
	if len(got) != 0 {
		t.Fatalf("Empty stream returned %d bytes, want 0", len(got))
	}
	got, ok = f.Stream("Real")
	if !ok || string(got) != "payload" {
		t.Fatalf("Real stream = %q, ok=%v", string(got), ok)
	}
}

func TestWriter_StreamLookupCaseInsensitive(t *testing.T) {
	w := cfb.NewWriter()
	if err := w.AddStream("WordDocument", []byte("doc")); err != nil {
		t.Fatalf("AddStream: %v", err)
	}
	raw, err := w.Bytes()
	if err != nil {
		t.Fatalf("Bytes: %v", err)
	}
	f, err := cfb.OpenBytes(raw)
	if err != nil {
		t.Fatalf("OpenBytes: %v", err)
	}
	for _, q := range []string{"WordDocument", "worddocument", "WORDDOCUMENT"} {
		if _, ok := f.Stream(q); !ok {
			t.Fatalf("Stream(%q): not found", q)
		}
	}
}

func TestWriter_DuplicateNameRejected(t *testing.T) {
	w := cfb.NewWriter()
	if err := w.AddStream("Foo", []byte("a")); err != nil {
		t.Fatalf("AddStream: %v", err)
	}
	if err := w.AddStream("foo", []byte("b")); err == nil {
		t.Fatal("expected error for duplicate name (case-insensitive), got nil")
	}
}

func TestWriter_RejectsEmptyName(t *testing.T) {
	w := cfb.NewWriter()
	if err := w.AddStream("", []byte("x")); err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestWriter_RejectsTooLongName(t *testing.T) {
	w := cfb.NewWriter()
	if err := w.AddStream(strings.Repeat("a", 32), []byte("x")); err == nil {
		t.Fatal("expected error for 32-character name")
	}
	if err := w.AddStream(strings.Repeat("a", 31), []byte("x")); err != nil {
		t.Fatalf("31-character name should be allowed: %v", err)
	}
}

func TestWriter_NoStreamsProducesValidEmptyContainer(t *testing.T) {
	w := cfb.NewWriter()
	raw, err := w.Bytes()
	if err != nil {
		t.Fatalf("Bytes: %v", err)
	}
	f, err := cfb.OpenBytes(raw)
	if err != nil {
		t.Fatalf("OpenBytes: %v", err)
	}
	if names := f.StreamNames(); len(names) != 0 {
		t.Fatalf("StreamNames = %v, want []", names)
	}
}

func TestWriter_RoundTripV4(t *testing.T) {
	w := cfb.NewWriter4K()
	if err := w.AddStream("EncryptionInfo", bytes.Repeat([]byte{0xCD}, 2048)); err != nil {
		t.Fatalf("AddStream: %v", err)
	}
	// Force a multi-FAT-sector layout by storing a payload that, even with
	// 4096-byte sectors, spans many sectors. Each FAT sector indexes 1024
	// sectors at v4, so use ~1100 sectors (~4.5 MB).
	big := make([]byte, 4500*1024)
	for i := range big {
		big[i] = byte(i * 31)
	}
	if err := w.AddStream("EncryptedPackage", big); err != nil {
		t.Fatalf("AddStream: %v", err)
	}
	raw, err := w.Bytes()
	if err != nil {
		t.Fatalf("Bytes: %v", err)
	}
	f, err := cfb.OpenBytes(raw)
	if err != nil {
		t.Fatalf("OpenBytes: %v", err)
	}
	got, ok := f.Stream("EncryptedPackage")
	if !ok || !bytes.Equal(got, big) {
		t.Fatalf("EncryptedPackage round-trip failed (ok=%v, len=%d)", ok, len(got))
	}
	got, ok = f.Stream("EncryptionInfo")
	if !ok || !bytes.Equal(got, bytes.Repeat([]byte{0xCD}, 2048)) {
		t.Fatalf("EncryptionInfo round-trip failed (ok=%v, len=%d)", ok, len(got))
	}
}

func TestWriter_NestedStorages(t *testing.T) {
	w := cfb.NewWriter()
	if err := w.AddStream("WordDocument", []byte("doc-payload")); err != nil {
		t.Fatalf("AddStream: %v", err)
	}
	if err := w.AddStream("Macros/VBA/Module1", []byte("module-1-bytes")); err != nil {
		t.Fatalf("AddStream nested: %v", err)
	}
	if err := w.AddStream("Macros/VBA/Module2", []byte("module-2-bytes")); err != nil {
		t.Fatalf("AddStream nested: %v", err)
	}
	if err := w.AddStream("Macros/dir", []byte("dir-bytes")); err != nil {
		t.Fatalf("AddStream nested: %v", err)
	}
	if err := w.AddStorage("EmptyStore"); err != nil {
		t.Fatalf("AddStorage: %v", err)
	}
	raw, err := w.Bytes()
	if err != nil {
		t.Fatalf("Bytes: %v", err)
	}
	f, err := cfb.OpenBytes(raw)
	if err != nil {
		t.Fatalf("OpenBytes: %v", err)
	}

	cases := map[string]string{
		"WordDocument":         "doc-payload",
		"Macros/VBA/Module1":   "module-1-bytes",
		"Macros/VBA/Module2":   "module-2-bytes",
		"Macros/dir":           "dir-bytes",
		"macros/vba/module1":   "module-1-bytes", // case-insensitive
	}
	for path, want := range cases {
		got, ok := f.StreamAt(path)
		if !ok {
			t.Fatalf("StreamAt(%q): not found", path)
		}
		if string(got) != want {
			t.Fatalf("StreamAt(%q) = %q, want %q", path, got, want)
		}
	}

	if _, ok := f.StreamAt("Macros"); ok {
		t.Fatal("StreamAt(\"Macros\") should miss; Macros is a storage, not a stream")
	}
	if _, ok := f.StreamAt("Macros/VBA/DoesNotExist"); ok {
		t.Fatal("StreamAt of missing path should be false")
	}

	if got := f.StorageNames(); len(got) != 2 {
		t.Fatalf("top-level storages = %v, want 2 (Macros + EmptyStore)", got)
	}
}

func TestWriter_AddStorageRejectsCollisionWithStream(t *testing.T) {
	w := cfb.NewWriter()
	if err := w.AddStream("Foo", []byte("x")); err != nil {
		t.Fatalf("AddStream: %v", err)
	}
	if err := w.AddStorage("Foo"); err == nil {
		t.Fatal("AddStorage on existing stream path should error")
	}
	if err := w.AddStream("Foo/Bar", []byte("y")); err == nil {
		t.Fatal("AddStream traversing through a stream should error")
	}
}

func TestWriter_RejectsIllegalPathCharacters(t *testing.T) {
	w := cfb.NewWriter()
	for _, bad := range []string{"a:b", "a\\b", "a!b"} {
		if err := w.AddStream(bad, []byte("x")); err == nil {
			t.Fatalf("AddStream(%q) should error", bad)
		}
	}
}

func TestWriter_RejectsEmptyPath(t *testing.T) {
	w := cfb.NewWriter()
	if err := w.AddStream("/", []byte("x")); err == nil {
		t.Fatal("AddStream(\"/\") should error")
	}
	if err := w.AddStream("a//b", []byte("x")); err == nil {
		t.Fatal("AddStream(\"a//b\") should error")
	}
}

func TestWriter_DeterministicOutput(t *testing.T) {
	build := func() []byte {
		w := cfb.NewWriter()
		_ = w.AddStream("Beta", []byte("b"))
		_ = w.AddStream("Alpha", []byte("a"))
		_ = w.AddStream("Gamma", bytes.Repeat([]byte{0x55}, 200))
		raw, err := w.Bytes()
		if err != nil {
			t.Fatalf("Bytes: %v", err)
		}
		return raw
	}
	if !bytes.Equal(build(), build()) {
		t.Fatal("Writer output is not deterministic for the same input")
	}
}
