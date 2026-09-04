package parts

import (
	"bytes"
	"fmt"
	"testing"
)

func TestCanonicalRelationshipsNaturalRIDOrderAndNoMutation(t *testing.T) {
	rels := &Relationships{Relationships: []*Relationship{
		{ID: "rId10", Type: "z", Target: "ten"},
		{ID: "rId2", Type: "a", Target: "two"},
		nil,
		{ID: "custom", Type: "b", Target: "custom"},
		{ID: "rId1", Type: "c", Target: "one"},
	}}
	got := canonicalRelationships(rels)
	want := []string{"rId1", "rId2", "rId10", "custom"}
	if len(got.Relationships) != len(want) {
		t.Fatalf("relationships = %d, want %d", len(got.Relationships), len(want))
	}
	for i, id := range want {
		if got.Relationships[i].ID != id {
			t.Fatalf("relationship[%d] = %q, want %q", i, got.Relationships[i].ID, id)
		}
	}
	if rels.Relationships[0].ID != "rId10" || rels.Relationships[1].ID != "rId2" {
		t.Fatal("canonicalization mutated source relationship order")
	}
}

func TestNormalizeOPCPartNameRejectsMalformedNames(t *testing.T) {
	valid := []string{"word/custom.xml", "/word/vendor/data.bin", "customXml/item1.xml"}
	for _, name := range valid {
		if _, ok := normalizeOPCPartName(name); !ok {
			t.Fatalf("valid part rejected: %q", name)
		}
	}
	invalid := []string{"", "../evil.bin", "word/../evil.bin", "word\\evil.bin", "word/./x.xml", "\x00bad"}
	for _, name := range invalid {
		if got, ok := normalizeOPCPartName(name); ok {
			t.Fatalf("malformed part accepted: %q -> %q", name, got)
		}
	}
}

func TestWritePackageDeterministicAcrossMapInsertionOrder(t *testing.T) {
	makePkg := func(reverse bool) *Package {
		p := &Package{
			Document:    &Document{Body: &Body{Paragraphs: []*Paragraph{{Runs: []*Run{{Text: []*Text{{Value: "deterministic"}}}}}}}},
			Headers:     map[string]*Header{},
			Footers:     map[string]*Footer{},
			OpaqueParts: map[string]OpaquePart{},
		}
		headerA := &Header{Paragraphs: []*Paragraph{{Runs: []*Run{{Text: []*Text{{Value: "A"}}}}}}}
		headerB := &Header{Paragraphs: []*Paragraph{{Runs: []*Run{{Text: []*Text{{Value: "B"}}}}}}}
		footerA := &Footer{Paragraphs: []*Paragraph{{Runs: []*Run{{Text: []*Text{{Value: "FA"}}}}}}}
		if reverse {
			p.Headers["word/header2.xml"] = headerB
			p.Headers["word/header1.xml"] = headerA
			p.Footers["word/footer1.xml"] = footerA
			p.OpaqueParts["custom/vendorB.bin"] = OpaquePart{ContentType: "application/octet-stream", Data: []byte("B")}
			p.OpaqueParts["custom/vendorA.bin"] = OpaquePart{ContentType: "application/octet-stream", Data: []byte("A")}
		} else {
			p.OpaqueParts["custom/vendorA.bin"] = OpaquePart{ContentType: "application/octet-stream", Data: []byte("A")}
			p.OpaqueParts["custom/vendorB.bin"] = OpaquePart{ContentType: "application/octet-stream", Data: []byte("B")}
			p.Footers["word/footer1.xml"] = footerA
			p.Headers["word/header1.xml"] = headerA
			p.Headers["word/header2.xml"] = headerB
		}
		return p
	}
	var a, b bytes.Buffer
	if err := WritePackage(makePkg(false), &a); err != nil {
		t.Fatal(err)
	}
	if err := WritePackage(makePkg(true), &b); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a.Bytes(), b.Bytes()) {
		t.Fatal("equivalent packages serialized to different bytes")
	}
}

func BenchmarkWritePackageLargeSynthetic(b *testing.B) {
	pkg := syntheticLargePackage(5000)
	b.ReportAllocs()
	b.SetBytes(int64(5000 * 80))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var out bytes.Buffer
		if err := WritePackage(pkg, &out); err != nil {
			b.Fatal(err)
		}
	}
}

func TestWritePackageLargeSyntheticCompletes(t *testing.T) {
	pkg := syntheticLargePackage(5000)
	var out bytes.Buffer
	if err := WritePackage(pkg, &out); err != nil {
		t.Fatalf("write large package: %v", err)
	}
	if out.Len() == 0 {
		t.Fatal("large package produced no output")
	}
}

func syntheticLargePackage(paragraphs int) *Package {
	body := &Body{Paragraphs: make([]*Paragraph, 0, paragraphs)}
	for i := 0; i < paragraphs; i++ {
		body.Paragraphs = append(body.Paragraphs, &Paragraph{Runs: []*Run{{Text: []*Text{{Value: fmt.Sprintf("paragraph %05d: deterministic production hardening payload", i)}}}}})
	}
	return &Package{Document: &Document{Body: body}}
}
