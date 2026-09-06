package stream

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"office/pkg/ooxml/rels"
	"office/pkg/word/parsers/ooxml/parts"
	wordresolve "office/pkg/word/parsers/ooxml/resolve"
)

func loadDOCXRenderMetadata(r io.ReaderAt, size int64) (*wordresolve.RenderMetadata, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, err
	}
	files := make(map[string]*zip.File, len(zr.File))
	for _, file := range zr.File {
		files[normalizeDOCXPartName(file.Name)] = file
	}

	rootData, err := readDOCXZipPart(files, "_rels/.rels")
	if err != nil {
		return nil, err
	}
	rootRels, err := rels.Parse(rootData)
	if err != nil {
		return nil, fmt.Errorf("word stream: parse package relationships: %w", err)
	}
	mainPart := ""
	for _, relationship := range rootRels.Relationships {
		if relationship.IsExternal() || !strings.HasSuffix(strings.ToLower(relationship.Type), "/officedocument") {
			continue
		}
		mainPart = rels.ResolveTarget("", relationship.Target)
		break
	}
	if mainPart == "" {
		return nil, fmt.Errorf("word stream: officeDocument relationship not found")
	}

	docRelsName := rels.RelationshipsPartName(mainPart)
	docRelsData, err := readDOCXZipPart(files, docRelsName)
	if err != nil {
		// A document without related metadata is legal; return empty defaults.
		return wordresolve.BuildRenderMetadata(nil, nil, nil, nil, nil), nil
	}
	docRels, err := rels.Parse(docRelsData)
	if err != nil {
		return nil, fmt.Errorf("word stream: parse document relationships: %w", err)
	}

	var styles *parts.Styles
	var numbering *parts.Numbering
	var theme *parts.Theme
	var settings *parts.Settings
	var fontTable *parts.FontTable

	for _, relationship := range docRels.Relationships {
		if relationship.IsExternal() {
			continue
		}
		target := rels.ResolveTarget(mainPart, relationship.Target)
		typeName := strings.ToLower(relationship.Type)
		switch {
		case strings.HasSuffix(typeName, "/styles"):
			value := &parts.Styles{}
			if err := unmarshalDOCXMetadataPart(files, target, value); err != nil {
				return nil, err
			}
			styles = value
		case strings.HasSuffix(typeName, "/numbering"):
			value := &parts.Numbering{}
			if err := unmarshalDOCXMetadataPart(files, target, value); err != nil {
				return nil, err
			}
			numbering = value
		case strings.HasSuffix(typeName, "/theme"):
			value := &parts.Theme{}
			if err := unmarshalDOCXMetadataPart(files, target, value); err != nil {
				return nil, err
			}
			theme = value
		case strings.HasSuffix(typeName, "/settings"):
			value := &parts.Settings{}
			if err := unmarshalDOCXMetadataPart(files, target, value); err != nil {
				return nil, err
			}
			settings = value
		case strings.HasSuffix(typeName, "/fonttable"):
			value := &parts.FontTable{}
			if err := unmarshalDOCXMetadataPart(files, target, value); err != nil {
				return nil, err
			}
			fontTable = value
		}
	}

	return wordresolve.BuildRenderMetadata(styles, numbering, theme, settings, fontTable), nil
}

func unmarshalDOCXMetadataPart(files map[string]*zip.File, name string, out any) error {
	data, err := readDOCXZipPart(files, name)
	if err != nil {
		return err
	}
	if err := xml.Unmarshal(data, out); err != nil {
		return fmt.Errorf("word stream: parse metadata part %q: %w", name, err)
	}
	return nil
}

func readDOCXZipPart(files map[string]*zip.File, name string) ([]byte, error) {
	file := files[normalizeDOCXPartName(name)]
	if file == nil {
		return nil, fmt.Errorf("word stream: DOCX part %q not found", name)
	}
	rc, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(rc)
}
