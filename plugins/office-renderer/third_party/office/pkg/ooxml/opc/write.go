package opc

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"sort"
	"strings"
)

// DocumentPart is a part that can serialize itself back to XML bytes.
// This is the commit point for OOXML write-back.
type DocumentPart interface {
	PartName() string
	ContentType() string
	Commit() ([]byte, error)
}

// AddPart adds or replaces a part in the package with the given data.
// The content type is registered as an Override if it cannot be inferred
// from the file extension.
func (p *Package) AddPart(name, contentType string, data []byte) {
	if p == nil {
		return
	}

	name = normalizePartName(name)
	p.parts[name] = &Part{
		Name:         name,
		OriginalName: name,
		ContentType:  contentType,
		Data:         data,
	}

	p.registerContentType(name, contentType)
}

// AddPartFromCommitter adds a DocumentPart to the package by calling its
// Commit method to obtain serialized bytes.
func (p *Package) AddPartFromCommitter(part DocumentPart) error {
	if p == nil || part == nil {
		return nil
	}

	data, err := part.Commit()
	if err != nil {
		return err
	}

	p.AddPart(part.PartName(), part.ContentType(), data)
	return nil
}

// RemovePart removes a part and its associated content type override.
func (p *Package) RemovePart(name string) {
	if p == nil {
		return
	}

	name = normalizePartName(name)
	delete(p.parts, name)

	if p.contentTypeOverrides != nil {
		delete(p.contentTypeOverrides, name)
	}
}

// Write serializes the package to a new ZIP archive.
// It writes all parts, reconstructs [Content_Types].xml from the current
// state, and preserves relationship parts.
func (p *Package) Write(w io.Writer) error {
	if p == nil {
		return io.ErrUnexpectedEOF
	}

	zw := zip.NewWriter(w)
	defer func() {
		_ = zw.Close()
	}()

	// Build a sorted list of part names for deterministic output.
	names := make([]string, 0, len(p.parts))
	for name := range p.parts {
		names = append(names, name)
	}
	sort.Strings(names)

	// Write [Content_Types].xml first (convention, not required).
	contentTypesData, err := p.buildContentTypesXML()
	if err != nil {
		return err
	}

	if err := writeZipEntry(zw, contentTypesPart, contentTypesData); err != nil {
		return err
	}

	// Write all other parts.
	for _, name := range names {
		if name == contentTypesPart {
			continue
		}
		part := p.parts[name]
		entryName := part.OriginalName
		if entryName == "" {
			entryName = name
		}
		if err := writeZipEntry(zw, entryName, part.Data); err != nil {
			return err
		}
	}

	return zw.Close()
}

// WriteToBytes is a convenience wrapper around Write that returns the
// serialized package as a byte slice.
func (p *Package) WriteToBytes() ([]byte, error) {
	var buf bytes.Buffer
	if err := p.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeZipEntry(zw *zip.Writer, name string, data []byte) error {
	// Use Store compression for very small parts to avoid overhead.
	method := zip.Deflate
	if len(data) < 64 {
		method = zip.Store
	}

	header := &zip.FileHeader{
		Name:   name,
		Method: method,
	}
	// Preserve modified time if needed; zero is acceptable for OOXML.
	w, err := zw.CreateHeader(header)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

func (p *Package) registerContentType(name, contentType string) {
	if p.contentTypeOverrides == nil {
		p.contentTypeOverrides = make(map[string]string)
	}

	// If the extension already has a default mapping for this content type,
	// we do not need an override.
	ext := extension(name)
	if ext != "" && p.contentTypes != nil {
		for _, def := range p.contentTypes.Defaults {
			if strings.EqualFold(def.Extension, ext) && def.ContentType == contentType {
				// Default already covers it.
				delete(p.contentTypeOverrides, name)
				return
			}
		}
	}

	p.contentTypeOverrides[name] = contentType
}

func (p *Package) buildContentTypesXML() ([]byte, error) {
	var ct ContentTypes

	// Start with existing defaults.
	if p.contentTypes != nil {
		ct.Defaults = append([]DefaultType(nil), p.contentTypes.Defaults...)
	}

	// Ensure common defaults are present.
	ensureDefault(&ct, "rels", "application/vnd.openxmlformats-package.relationships+xml")
	ensureDefault(&ct, "xml", "application/xml")

	// Build overrides from current state.
	overrideNames := make([]string, 0, len(p.contentTypeOverrides))
	for name := range p.contentTypeOverrides {
		overrideNames = append(overrideNames, name)
	}
	sort.Strings(overrideNames)

	for _, name := range overrideNames {
		ct.Overrides = append(ct.Overrides, OverrideType{
			PartName:    "/" + name,
			ContentType: p.contentTypeOverrides[name],
		})
	}

	var buf bytes.Buffer
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	enc := xml.NewEncoder(&buf)
	enc.Indent("", "  ")
	if err := enc.Encode(ct); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func ensureDefault(ct *ContentTypes, ext, contentType string) {
	for _, d := range ct.Defaults {
		if strings.EqualFold(d.Extension, ext) {
			return
		}
	}
	ct.Defaults = append(ct.Defaults, DefaultType{
		Extension:   ext,
		ContentType: contentType,
	})
}
