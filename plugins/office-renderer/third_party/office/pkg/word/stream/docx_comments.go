package stream

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"

	wordir "office/pkg/word/ir"
)

type docxCommentLoader struct {
	parts map[string]*zip.File
	part  string
	cache map[int]wordir.Story
	loaded bool
}

func newDOCXCommentLoader(r io.ReaderAt, size int64) (*docxCommentLoader, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, err
	}
	parts := make(map[string]*zip.File, len(zr.File))
	for _, f := range zr.File {
		parts[normalizeDOCXPartName(f.Name)] = f
	}
	loader := &docxCommentLoader{parts: parts, cache: make(map[int]wordir.Story)}
	mainPart, err := findDOCXMainPart(parts)
	if err != nil {
		mainPart = "word/document.xml"
	}
	relsName := normalizeDOCXPartName(path.Join(path.Dir(mainPart), "_rels", path.Base(mainPart)+".rels"))
	if rels := parts[relsName]; rels != nil {
		rc, err := rels.Open()
		if err != nil {
			return nil, err
		}
		dec := xml.NewDecoder(rc)
		for {
			token, err := dec.Token()
			if err != nil {
				break
			}
			start, ok := token.(xml.StartElement)
			if !ok || start.Name.Local != "Relationship" {
				continue
			}
			if strings.HasSuffix(strings.ToLower(docxAttr(start, "Type")), "/comments") && strings.ToLower(docxAttr(start, "TargetMode")) != "external" {
				loader.part = normalizeDOCXPartName(path.Join(path.Dir(mainPart), docxAttr(start, "Target")))
				break
			}
		}
		rc.Close()
	}
	if loader.part == "" && parts["word/comments.xml"] != nil {
		loader.part = "word/comments.xml"
	}
	return loader, nil
}

func (l *docxCommentLoader) Load(id int) (wordir.Story, bool, error) {
	if l == nil {
		return wordir.Story{}, false, nil
	}
	if !l.loaded {
		if err := l.loadAll(); err != nil {
			return wordir.Story{}, false, err
		}
	}
	story, ok := l.cache[id]
	return story, ok, nil
}

func (l *docxCommentLoader) loadAll() error {
	l.loaded = true
	if l.part == "" {
		return nil
	}
	file := l.parts[normalizeDOCXPartName(l.part)]
	if file == nil {
		return fmt.Errorf("word stream: comments part %q not found", l.part)
	}
	rc, err := file.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	dec := xml.NewDecoder(rc)
	for {
		token, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "comment" {
			continue
		}
		id, err := strconv.Atoi(strings.TrimSpace(docxAttr(start, "id")))
		if err != nil || id < 0 {
			if err := dec.Skip(); err != nil {
				return err
			}
			continue
		}
		story, err := parseDOCXNoteStory(dec, start, 0)
		if err != nil {
			return err
		}
		story.Author = docxAttr(start, "author")
		story.Date = docxAttr(start, "date")
		l.cache[id] = story
	}
}
