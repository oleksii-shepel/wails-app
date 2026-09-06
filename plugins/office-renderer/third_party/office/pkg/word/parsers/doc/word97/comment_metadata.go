package word97

import (
	"encoding/binary"
	"fmt"
	"unicode/utf16"
)

// applyCanonicalCommentMetadata projects ATRDPre10 author/date metadata from
// PlcfAtnBkf onto already-materialized comment stories. The binary record is a
// Word97 concern, so compatibility adapters no longer need their own decoder.
func applyCanonicalCommentMetadata(ctx *Context, opened *OpenCore) {
	if ctx == nil || opened == nil || len(ctx.Comments) == 0 {
		return
	}
	authors, dates := loadCommentMetadataReaderAt(opened, len(ctx.Comments))
	if len(authors) == 0 && len(dates) == 0 {
		return
	}
	for id, story := range ctx.Comments {
		if author := authors[id]; author != "" {
			story.Author = author
		}
		if date := dates[id]; date != "" {
			story.Date = date
		}
		ctx.Comments[id] = story
	}
}

func loadCommentMetadataReaderAt(opened *OpenCore, commentCount int) (map[int]string, map[int]string) {
	if opened == nil || commentCount <= 0 {
		return nil, nil
	}
	const (
		plcfAtnBkfIndex = 4
		atrdPre10Size    = 30
		xstSize          = 22
	)
	blob, ok := readTableRange(opened.Table, opened.TableSize, opened.FIB, opened.Layout, plcfAtnBkfIndex, 4+atrdPre10Size)
	if !ok {
		return nil, nil
	}
	n := (len(blob) - 4) / (4 + atrdPre10Size)
	if n <= 0 {
		return nil, nil
	}
	if n > commentCount {
		n = commentCount
	}
	dataOffset := (n + 1) * 4
	if dataOffset+n*atrdPre10Size > len(blob) {
		return nil, nil
	}
	authors := make(map[int]string, n)
	dates := make(map[int]string, n)
	for i := 0; i < n; i++ {
		rec := blob[dataOffset+i*atrdPre10Size : dataOffset+(i+1)*atrdPre10Size]
		cch := int(binary.LittleEndian.Uint16(rec[0:2]))
		if cch > 10 {
			cch = 10
		}
		if cch > 0 && 2+cch*2 <= len(rec) {
			u16 := make([]uint16, cch)
			for j := 0; j < cch; j++ {
				u16[j] = binary.LittleEndian.Uint16(rec[2+j*2 : 4+j*2])
			}
			authors[i+1] = string(utf16.Decode(u16))
		}
		dttm := binary.LittleEndian.Uint32(rec[xstSize+4 : xstSize+8])
		if dttm == 0 {
			continue
		}
		day := int((dttm >> 11) & 0x1F)
		month := int((dttm >> 16) & 0xF)
		year := int((dttm>>20)&0x1FF) + 1900
		if year > 1900 && month >= 1 && month <= 12 && day >= 1 && day <= 31 {
			dates[i+1] = fmt.Sprintf("%04d-%02d-%02d", year, month, day)
		}
	}
	if len(authors) == 0 {
		authors = nil
	}
	if len(dates) == 0 {
		dates = nil
	}
	return authors, dates
}
