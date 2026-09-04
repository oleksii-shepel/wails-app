package escher

import "fmt"

// Walk visits records depth-first. Containers are visited before their
// children. Returning an error stops traversal.
func Walk(records []Record, visit func(Record) error) error {
	for _, rec := range records {
		if visit != nil {
			if err := visit(rec); err != nil {
				return err
			}
		}
		if len(rec.Children) > 0 {
			if err := Walk(rec.Children, visit); err != nil {
				return err
			}
		}
	}
	return nil
}

// WalkStream parses and walks one contiguous Escher stream.
func WalkStream(data []byte, visit func(Record) error) error {
	records, err := ParseRecords(data)
	if err != nil {
		return err
	}
	return Walk(records, visit)
}

// Scan finds structurally valid Escher records embedded in a larger application
// stream. It is intentionally tolerant: malformed candidates are skipped one
// byte at a time. This is useful for WordDocument/Table/Data streams where
// OfficeArt is embedded among unrelated binary structures.
func Scan(data []byte, accept func(Header) bool) []Record {
	if len(data) < HeaderSize {
		return nil
	}
	var out []Record
	for off := 0; off+HeaderSize <= len(data); off++ {
		h, err := ParseHeader(data[off:])
		if err != nil {
			continue
		}
		if accept != nil && !accept(h) {
			continue
		}
		end64 := uint64(off) + HeaderSize + uint64(h.Length)
		if end64 > uint64(len(data)) {
			continue
		}
		end := int(end64)
		payload := data[off+HeaderSize : end]
		rec := Record{
			Header: h,
			Offset: off,
			Data:   append([]byte(nil), payload...),
		}
		if h.IsContainer() {
			children, err := ParseRecords(payload)
			if err != nil {
				continue
			}
			rec.Children = children
		}
		out = append(out, rec)
		// Avoid rediscovering child records as top-level scan hits.
		off = end - 1
	}
	return out
}

// PropertyTable parses FOPT data and returns a map with "last property wins"
// semantics, matching OfficeArt's primary/secondary/tertiary overlay behavior.
func PropertyTable(data []byte, count int) (map[uint16]Property, error) {
	props, err := ParseFOPT(data, count)
	if err != nil {
		return nil, fmt.Errorf("escher: property table: %w", err)
	}
	return PropertyMap(props), nil
}
