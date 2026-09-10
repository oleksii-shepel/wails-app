package opc

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"os"
	"sort"
	"strings"

	"office/pkg/ooxml/rels"
)

const (
	contentTypesPart = "[content_types].xml"

	typeOfficeDocument       = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument"
	typeOfficeDocumentStrict = "http://purl.oclc.org/ooxml/officeDocument/relationships/officeDocument"

	wordMainContentType              = "application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"
	wordTemplateMainContentType      = "application/vnd.openxmlformats-officedocument.wordprocessingml.template.main+xml"
	wordMacroMainContentType         = "application/vnd.openxmlformats-officedocument.wordprocessingml.document.macroEnabled.main+xml"
	wordMacroTemplateMainContentType = "application/vnd.openxmlformats-officedocument.wordprocessingml.template.macroEnabled.main+xml"
	spreadsheetMainContentType       = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"
	presentationMainContentType      = "application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"
)

// OpenOptions controls OPC package opening.
type OpenOptions struct {
	// Password decrypts an OOXML package stored inside an OLE container.
	// The zero value still permits the ECMA-376 default password where applicable.
	Password string
}

// PackageType identifies the primary office format represented by an OPC package.
type PackageType string

const (
	PackageTypeUnknown      PackageType = "unknown"
	PackageTypeWord         PackageType = "docx"
	PackageTypeSpreadsheet  PackageType = "xlsx"
	PackageTypePresentation PackageType = "pptx"
)

// Part is a single named OPC part.
type Part struct {
	Name         string
	OriginalName string
	ContentType  string
	Data         []byte
}

// Package is a normalized in-memory view of an OPC ZIP archive.
type Package struct {
	parts                map[string]*Part
	contentTypes         *ContentTypes
	contentTypeOverrides map[string]string
	size                 int64
	packageType          PackageType
}

// NewPackage creates an empty OPC package suitable for building from scratch.
func NewPackage() *Package {
	return &Package{
		parts:                make(map[string]*Part),
		contentTypes:         &ContentTypes{},
		contentTypeOverrides: make(map[string]string),
	}
}

// Open reads an OPC package from disk.
func Open(path string) (*Package, error) {
	return OpenWithOptions(path, OpenOptions{})
}

// OpenWithOptions reads an OPC package from disk using per-call options.
func OpenWithOptions(path string, options OpenOptions) (*Package, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = file.Close()
	}()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}

	return OpenReaderAtWithOptions(file, info.Size(), options)
}

// OpenReaderAt reads an OPC package from a reader/size pair.
func OpenReaderAt(r io.ReaderAt, size int64) (*Package, error) {
	return OpenReaderAtWithOptions(r, size, OpenOptions{})
}

// OpenReaderAtWithOptions reads an OPC package from a reader/size pair using per-call options.
func OpenReaderAtWithOptions(r io.ReaderAt, size int64, options OpenOptions) (*Package, error) {
	pkg, err := openZipReaderAt(r, size)
	if err == nil {
		return pkg, nil
	}

	data := make([]byte, size)
	if _, readErr := io.ReadFull(io.NewSectionReader(r, 0, size), data); readErr != nil {
		return nil, readErr
	}

	return openContainerBytes(data, err, options)
}

func openZipReaderAt(r io.ReaderAt, size int64) (*Package, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, err
	}
	return openZipArchive(zr, size)
}

func openZipArchive(zr *zip.Reader, size int64) (*Package, error) {
	pkg := &Package{
		parts:                make(map[string]*Part, len(zr.File)),
		contentTypeOverrides: make(map[string]string),
		size:                 size,
	}

	for _, file := range zr.File {
		if file.FileInfo().IsDir() {
			continue
		}

		data, err := readZipFile(file)
		if err != nil {
			return nil, err
		}

		name := normalizePartName(file.Name)
		if name == "" {
			continue
		}

		pkg.parts[name] = &Part{
			Name:         name,
			OriginalName: file.Name,
			Data:         data,
		}
	}

	if err := pkg.loadContentTypes(); err != nil {
		return nil, err
	}

	for _, part := range pkg.parts {
		part.ContentType = pkg.ContentType(part.Name)
	}

	pkg.packageType = pkg.detectPackageType()
	return pkg, nil
}

// OpenBytes reads an OPC package from an in-memory byte slice.
func OpenBytes(data []byte) (*Package, error) {
	return OpenBytesWithOptions(data, OpenOptions{})
}

// OpenBytesWithOptions reads an OPC package from memory using per-call options.
func OpenBytesWithOptions(data []byte, options OpenOptions) (*Package, error) {
	if pkg, err := openZipReaderAt(bytes.NewReader(data), int64(len(data))); err == nil {
		return pkg, nil
	} else {
		return openContainerBytes(data, err, options)
	}
}

// OpenStream reads an OPC package from a stream by spooling it to a temporary
// file first. This avoids holding the original input stream in memory before
// the package loader expands the individual parts.
func OpenStream(r io.Reader) (*Package, error) {
	return OpenStreamWithOptions(r, OpenOptions{})
}

// OpenStreamWithOptions reads an OPC package from a stream using per-call options.
func OpenStreamWithOptions(r io.Reader, options OpenOptions) (*Package, error) {
	file, err := os.CreateTemp("", "office-opc-stream-*")
	if err != nil {
		return nil, err
	}
	name := file.Name()
	defer func() {
		_ = os.Remove(name)
	}()
	defer func() {
		_ = file.Close()
	}()

	size, err := io.Copy(file, r)
	if err != nil {
		return nil, err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	return OpenReaderAtWithOptions(file, size, options)
}

// Size returns the original package size in bytes.
func (p *Package) Size() int64 {
	if p == nil {
		return 0
	}
	return p.size
}

// PackageType returns the detected office package type.
func (p *Package) PackageType() PackageType {
	if p == nil {
		return PackageTypeUnknown
	}
	return p.packageType
}

// ContentTypes returns the parsed [Content_Types].xml part.
func (p *Package) ContentTypes() *ContentTypes {
	if p == nil {
		return nil
	}
	return p.contentTypes
}

// Parts returns all package parts sorted by normalized name.
func (p *Package) Parts() []Part {
	if p == nil {
		return nil
	}

	names := make([]string, 0, len(p.parts))
	for name := range p.parts {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]Part, 0, len(names))
	for _, name := range names {
		part := p.parts[name]
		out = append(out, *part)
	}
	return out
}

// Part looks up a package part by normalized path.
func (p *Package) Part(name string) (Part, bool) {
	if p == nil {
		return Part{}, false
	}

	part, ok := p.parts[normalizePartName(name)]
	if !ok {
		return Part{}, false
	}
	return *part, true
}

// ContentType resolves the content type for a part.
func (p *Package) ContentType(name string) string {
	if p == nil || p.contentTypes == nil {
		return ""
	}

	name = normalizePartName(name)
	if contentType, ok := p.contentTypeOverrides[name]; ok {
		return contentType
	}

	ext := extension(name)
	if ext == "" {
		return ""
	}

	for _, def := range p.contentTypes.Defaults {
		if strings.EqualFold(def.Extension, ext) {
			return def.ContentType
		}
	}

	return ""
}

// Relationships returns the relationship document for the given source part.
// Use an empty source part to access package root relationships.
func (p *Package) Relationships(sourcePart string) (*rels.Document, bool, error) {
	if p == nil {
		return nil, false, nil
	}

	relsName := rels.RelationshipsPartName(sourcePart)
	part, ok := p.parts[normalizePartName(relsName)]
	if !ok {
		return nil, false, nil
	}

	doc, err := rels.Parse(part.Data)
	if err != nil {
		return nil, true, err
	}
	return doc, true, nil
}

// ResolveRelationship resolves a relationship target from a source part into another package part.
func (p *Package) ResolveRelationship(sourcePart, relationshipID string) (Part, rels.Relationship, bool, error) {
	doc, ok, err := p.Relationships(sourcePart)
	if err != nil || !ok {
		return Part{}, rels.Relationship{}, false, err
	}

	relationship, found := doc.ByID(relationshipID)
	if !found || relationship.IsExternal() {
		return Part{}, relationship, false, nil
	}

	targetName := rels.ResolveTarget(sourcePart, relationship.Target)
	part, found := p.Part(targetName)
	return part, relationship, found, nil
}

func (p *Package) loadContentTypes() error {
	part, ok := p.parts[contentTypesPart]
	if !ok {
		return io.ErrUnexpectedEOF
	}

	var contentTypes ContentTypes
	if err := xml.Unmarshal(part.Data, &contentTypes); err != nil {
		return err
	}

	p.contentTypes = &contentTypes
	for _, override := range contentTypes.Overrides {
		p.contentTypeOverrides[normalizePartName(override.PartName)] = override.ContentType
	}

	return nil
}

func (p *Package) detectPackageType() PackageType {
	rootRels, ok, err := p.Relationships("")
	if err != nil || !ok {
		return PackageTypeUnknown
	}

	for _, rel := range rootRels.Relationships {
		if (rel.Type != typeOfficeDocument && rel.Type != typeOfficeDocumentStrict) || rel.IsExternal() {
			continue
		}

		mainPartName := rels.ResolveTarget("", rel.Target)
		switch p.ContentType(mainPartName) {
		case wordMainContentType, wordTemplateMainContentType, wordMacroMainContentType, wordMacroTemplateMainContentType:
			return PackageTypeWord
		case spreadsheetMainContentType:
			return PackageTypeSpreadsheet
		case presentationMainContentType:
			return PackageTypePresentation
		}
	}

	return PackageTypeUnknown
}

func readZipFile(file *zip.File) ([]byte, error) {
	rc, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rc.Close()
	}()

	return io.ReadAll(rc)
}

func normalizePartName(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	name = strings.TrimPrefix(name, "./")
	name = strings.TrimPrefix(name, "/")
	name = strings.TrimSpace(name)
	return strings.ToLower(name)
}

func extension(name string) string {
	idx := strings.LastIndexByte(name, '.')
	if idx < 0 || idx == len(name)-1 {
		return ""
	}
	return name[idx+1:]
}
