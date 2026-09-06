package word97

import (
	"errors"
	"testing"

	wordir "office/pkg/word/ir"
	docbinary "office/pkg/word/parsers/doc/binary"
)

func TestValidateContextAcceptsResolvedSideStories(t *testing.T) {
	opened := &OpenCore{Metadata: Metadata{
		StoryCounts: docbinary.StoryCounts{Ftn: 1, Edn: 1, Atn: 1, Hdr: 1},
		FootnoteRefs: map[int]int{1: 1},
		EndnoteRefs: map[int]int{2: 1},
		CommentRefs: map[int]int{3: 1},
	}}
	context := &Context{
		Footnotes: map[int]wordir.Story{1: {}},
		Endnotes:  map[int]wordir.Story{1: {}},
		Comments:  map[int]wordir.Story{1: {}},
		Headers:   map[string]wordir.Story{"default": {}},
	}
	if err := ValidateContext(opened, context); err != nil {
		t.Fatalf("ValidateContext() error = %v", err)
	}
}

func TestValidateContextReportsCanonicalUnsupportedFeature(t *testing.T) {
	tests := []struct {
		name    string
		counts  docbinary.StoryCounts
		meta    func(*Metadata)
		context *Context
		want    string
	}{
		{name: "footnote", counts: docbinary.StoryCounts{Ftn: 1}, context: &Context{}, want: "footnote story"},
		{name: "endnote", counts: docbinary.StoryCounts{Edn: 1}, context: &Context{}, want: "endnote story"},
		{name: "comment", counts: docbinary.StoryCounts{Atn: 1}, context: &Context{}, want: "comment story"},
		{name: "header", counts: docbinary.StoryCounts{Hdr: 1}, context: &Context{}, want: "header/footer story"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			metadata := Metadata{StoryCounts: tc.counts}
			if tc.meta != nil {
				tc.meta(&metadata)
			}
			err := ValidateContext(&OpenCore{Metadata: metadata}, tc.context)
			var unsupported UnsupportedError
			if !errors.As(err, &unsupported) {
				t.Fatalf("ValidateContext() error = %T %v, want UnsupportedError", err, err)
			}
			if unsupported.Feature != tc.want {
				t.Fatalf("UnsupportedError.Feature = %q, want %q", unsupported.Feature, tc.want)
			}
		})
	}
}
