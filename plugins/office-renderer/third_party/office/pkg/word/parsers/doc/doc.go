package doc

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"office/pkg/ooxml/cfb"
	wordevent "office/pkg/word/event"
	wordir "office/pkg/word/ir"
	pre97 "office/pkg/word/parsers/doc/pre97"
	word97 "office/pkg/word/parsers/doc/word97"
	legacy "office/pkg/word/parsers/legacy"
)

// StreamEventType, StreamEvent and StreamHandler remain public aliases for the
// shared Word semantic event transport.
type StreamEventType = wordevent.Type

const (
	StreamStartDocument     = wordevent.StartDocument
	StreamEndDocument       = wordevent.EndDocument
	StreamStartParagraph    = wordevent.StartParagraph
	StreamEndParagraph      = wordevent.EndParagraph
	StreamStartTable        = wordevent.StartTable
	StreamEndTable          = wordevent.EndTable
	StreamStartTableRow     = wordevent.StartTableRow
	StreamEndTableRow       = wordevent.EndTableRow
	StreamStartTableCell    = wordevent.StartTableCell
	StreamEndTableCell      = wordevent.EndTableCell
	StreamText              = wordevent.Text
	StreamTab               = wordevent.Tab
	StreamBreak             = wordevent.Break
	StreamDrawing           = wordevent.Drawing
	StreamEmbeddedObject    = wordevent.EmbeddedObject
	StreamBookmarkStart     = wordevent.BookmarkStart
	StreamBookmarkEnd       = wordevent.BookmarkEnd
	StreamFootnoteReference = wordevent.FootnoteReference
	StreamEndnoteReference  = wordevent.EndnoteReference
	StreamCommentReference  = wordevent.CommentReference
	StreamSection           = wordevent.Section
)

type StreamUnsupportedError struct{ Feature string }
func (e StreamUnsupportedError) Error() string { return fmt.Sprintf("doc stream: unsupported %s", e.Feature) }

type StreamEvent = wordevent.Event
type StreamHandler = wordevent.Handler

func WalkFile(path string, handler StreamHandler) error {
	if handler == nil { return fmt.Errorf("doc stream: handler cannot be nil") }
	f, err := cfb.OpenFileBacked(path)
	if err != nil { return fmt.Errorf("doc stream: open cfb: %w", err) }
	defer f.Close()
	if handled, err := walkWord97CFB(f, handler); handled || err != nil { return err }
	if handled, err := walkPre97CFB(f, handler); handled || err != nil { return err }
	return translateStreamError(legacy.WalkOpenedCFB(f, handler))
}

func WalkBytes(data []byte, handler StreamHandler) error {
	if handler == nil { return fmt.Errorf("doc stream: handler cannot be nil") }
	f, err := cfb.OpenBytes(data)
	if err != nil { return fmt.Errorf("doc stream: open cfb: %w", err) }
	if handled, err := walkWord97CFB(f, handler); handled || err != nil { return err }
	if handled, err := walkPre97CFB(f, handler); handled || err != nil { return err }
	return translateStreamError(legacy.WalkOpenedCFB(f, handler))
}

// openWord97CFB owns normal unencrypted Word97+ format selection and canonical
// opening. Streaming and full IR materialization share this one open boundary.
func openWord97CFB(f *cfb.File) (bool, word97.Cursor, *word97.Context, *word97.OpenCore, error) {
	if f == nil {
		return false, nil, nil, nil, nil
	}
	wordReader, wordSize, ok, readErr := f.StreamReaderAt("WordDocument")
	if !ok && readErr == nil { wordReader, wordSize, ok, readErr = f.StreamReaderAt("Ole") }
	if readErr != nil { return true, nil, nil, nil, fmt.Errorf("doc stream: open WordDocument reader: %w", readErr) }
	if !ok || wordSize < 18 { return false, nil, nil, nil, nil }
	header := make([]byte, 18)
	n, readErr := wordReader.ReadAt(header, 0)
	if readErr != nil && !errors.Is(readErr, io.EOF) { return true, nil, nil, nil, readErr }
	if n != len(header) { return false, nil, nil, nil, nil }
	fib := classifyFIB(header)
	if !fib.word97OrNewer() || fib.Encrypted { return false, nil, nil, nil, nil }

	cursor, context, opened, err := word97.OpenReaderAt(wordReader, wordSize, f)
	if err != nil { return true, nil, context, opened, err }
	return true, cursor, context, opened, nil
}

// walkWord97CFB owns normal unencrypted Word97+ dispatch. The complete read
// pipeline, including drawing/media projection, lives in the canonical word97
// package. This layer only selects the format and translates public errors.
func walkWord97CFB(f *cfb.File, handler StreamHandler) (bool, error) {
	handled, cursor, context, _, err := openWord97CFB(f)
	if !handled || err != nil {
		return handled, translateStreamError(err)
	}
	return true, translateStreamError(word97.Walk(cursor, context, handler))
}

func materializeWord97CFB(f *cfb.File, name string) (*wordir.Document, bool, error) {
	handled, cursor, context, opened, err := openWord97CFB(f)
	if !handled || err != nil {
		return nil, handled, translateStreamError(err)
	}
	m := newMaterializer(name)
	if err := word97.Walk(cursor, context, m.Handle); err != nil {
		return nil, true, translateStreamError(err)
	}
	doc := m.Document()
	applySequentialDocumentSourceCore(f, opened, doc)
	return doc, true, nil
}

// walkPre97CFB gives Word 6/95 its own canonical composition boundary. The
// mature decoder still lives behind the compatibility port, but pre-Word97 no
// longer enters through the generic legacy stream contract.
func walkPre97CFB(f *cfb.File, handler StreamHandler) (bool, error) {
	err := pre97.WalkOpenedCFB(f, legacy.PreWord97Compatibility{}, handler)
	if errors.Is(err, pre97.ErrNotApplicable) {
		return false, nil
	}
	return true, translateStreamError(err)
}

func translateStreamError(err error) error {
	if err == nil { return nil }
	var canonicalUnsupported word97.UnsupportedError
	if errors.As(err, &canonicalUnsupported) { return StreamUnsupportedError{Feature:canonicalUnsupported.Feature} }
	var pre97Unsupported pre97.UnsupportedError
	if errors.As(err, &pre97Unsupported) { return StreamUnsupportedError{Feature:pre97Unsupported.Feature} }
	var compatibilityUnsupported legacy.StreamUnsupportedError
	if errors.As(err, &compatibilityUnsupported) { return StreamUnsupportedError{Feature:compatibilityUnsupported.Feature} }
	return err
}

// ParseLossless materializes a file-backed DOC by consuming the same sequential
// ReaderAt stream used by bounded layout. Normal Word97+ uses one canonical open
// for both event materialization and lossless document-wide source projection.
func ParseLossless(path string) (*wordir.Document, error) {
	name := filepath.Base(path)
	if f, openErr := cfb.OpenFileBacked(path); openErr == nil {
		doc, handled, err := materializeWord97CFB(f, name)
		_ = f.Close()
		if handled {
			return doc, err
		}
	}
	m:=newMaterializer(name)
	if err:=WalkFile(path,m.Handle);err==nil{
		return m.Document(),nil
	}else if sequentialRequiredPath(path){
		return nil,err
	}
	return legacy.ParseDocLossless(path)
}

// ParseBytesLossless materializes the stable IR from sequential events. Normal
// Word97+ uses one canonical compound-file/open snapshot; legacy whole-document
// parsing remains only for malformed/encrypted compatibility inputs.
func ParseBytesLossless(data []byte, name string) (*wordir.Document, error) {
	name = filepath.Base(strings.TrimSpace(name))
	if name == "." || name == "" { name = "document.doc" }
	if f, openErr := cfb.OpenBytes(data); openErr == nil {
		if doc, handled, err := materializeWord97CFB(f, name); handled {
			return doc, err
		}
	}
	m := newMaterializer(name)
	if err := WalkBytes(data, m.Handle); err == nil {
		return m.Document(), nil
	} else if sequentialRequired(data) {
		// Normal unencrypted Word97+ must not silently fall back to a second
		// materializing parser. A failure here is a real sequential-parser gap.
		return nil, err
	}
	return legacy.ParseDocBytesLossless(data, name)
}

// sequentialRequired identifies normal unencrypted Word97+ files. These files
// are architecturally required to materialize through the sequential parser.
func sequentialRequired(data []byte) bool {
	f, err := cfb.OpenBytes(data)
	if err != nil { return false }
	return sequentialRequiredCFB(f)
}

func sequentialRequiredPath(path string) bool {
	f,err:=cfb.OpenFileBacked(path);if err!=nil{return false};defer f.Close()
	return sequentialRequiredCFB(f)
}

func sequentialRequiredCFB(f *cfb.File) bool {
	if f==nil{return false}
	word,size,ok,err:=f.StreamReaderAt("WordDocument")
	if !ok&&err==nil{word,size,ok,err=f.StreamReaderAt("Ole")}
	if err!=nil||!ok||word==nil||size<18{return false}
	header:=make([]byte,18);n,err:=word.ReadAt(header,0);if err!=nil&&!errors.Is(err,io.EOF){return false};if n!=len(header){return false}
	fib:=classifyFIB(header);return fib.word97OrNewer()&&!fib.Encrypted
}
