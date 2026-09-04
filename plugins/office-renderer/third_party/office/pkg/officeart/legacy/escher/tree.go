package escher

import (
	"encoding/binary"
	"fmt"
)

// ShapeNode is the shared structural result of parsing an SpContainer.
// It deliberately does not interpret application-specific client data.
type ShapeNode struct {
	ShapeID   uint32
	ShapeType uint16
	Flags     uint32

	Properties []Property

	HasClientTextbox bool
	ClientTextbox    []byte
	ClientAnchor     []byte
	ChildAnchor      []byte
	ClientData       []byte

	GroupRect *GroupRect
	Children  []*ShapeNode
}

type GroupRect struct {
	Left, Top, Right, Bottom int32
}

// ShapeTree walks an Escher record tree and returns every top-level shape or
// group represented by SpContainer nodes.
func ShapeTree(records []Record) ([]*ShapeNode, error) {
	var out []*ShapeNode
	for i := range records {
		nodes, err := collectShapeNodes(records[i])
		if err != nil {
			return nil, err
		}
		out = append(out, nodes...)
	}
	return out, nil
}

func collectShapeNodes(rec Record) ([]*ShapeNode, error) {
	if rec.Header.Type == RecordSpContainer {
		node, err := parseSpContainer(rec)
		if err != nil {
			return nil, err
		}
		return []*ShapeNode{node}, nil
	}
	var out []*ShapeNode
	for _, child := range rec.Children {
		nodes, err := collectShapeNodes(child)
		if err != nil {
			return nil, err
		}
		out = append(out, nodes...)
	}
	return out, nil
}

func parseSpContainer(rec Record) (*ShapeNode, error) {
	node := &ShapeNode{}
	for _, child := range rec.Children {
		switch child.Header.Type {
		case RecordFSP:
			if len(child.Data) < 8 {
				return nil, fmt.Errorf("escher: FSP at %d is %d bytes, want >= 8", child.Offset, len(child.Data))
			}
			node.ShapeID = binary.LittleEndian.Uint32(child.Data[0:4])
			node.Flags = binary.LittleEndian.Uint32(child.Data[4:8])
			node.ShapeType = child.Header.Instance
		case RecordFSPGR:
			if len(child.Data) >= 16 {
				node.GroupRect = &GroupRect{
					Left:   int32(binary.LittleEndian.Uint32(child.Data[0:4])),
					Top:    int32(binary.LittleEndian.Uint32(child.Data[4:8])),
					Right:  int32(binary.LittleEndian.Uint32(child.Data[8:12])),
					Bottom: int32(binary.LittleEndian.Uint32(child.Data[12:16])),
				}
			}
		case RecordFOPT, RecordSecondaryFOPT, RecordTertiaryFOPT:
			props, err := ParseFOPT(child.Data, int(child.Header.Instance))
			if err != nil {
				return nil, fmt.Errorf("escher: shape %d FOPT: %w", node.ShapeID, err)
			}
			node.Properties = append(node.Properties, props...)
		case RecordClientTextbox:
			node.HasClientTextbox = true
			node.ClientTextbox = append([]byte(nil), child.Data...)
		case RecordClientAnchor:
			node.ClientAnchor = append([]byte(nil), child.Data...)
		case RecordChildAnchor:
			node.ChildAnchor = append([]byte(nil), child.Data...)
		case RecordClientData:
			node.ClientData = append([]byte(nil), child.Data...)
		case RecordSpgrContainer:
			children, err := ShapeTree(child.Children)
			if err != nil {
				return nil, err
			}
			node.Children = append(node.Children, children...)
		}
	}
	return node, nil
}

// ParseShapeContainerPayload parses the child-record payload of one
// OfficeArtSpContainer. This is the form exposed directly by BIFF and PPT
// application record walkers.
func ParseShapeContainerPayload(data []byte) (*ShapeNode, error) {
	children, err := ParseRecords(data)
	if err != nil {
		return nil, err
	}
	return parseSpContainer(Record{
		Header:   Header{Version: ContainerVersion, Type: RecordSpContainer},
		Children: children,
	})
}

// ParseShapeContainerRecord parses one complete OfficeArtSpContainer record.
func ParseShapeContainerRecord(data []byte) (*ShapeNode, error) {
	records, err := ParseRecords(data)
	if err != nil {
		return nil, err
	}
	if len(records) != 1 || records[0].Header.Type != RecordSpContainer {
		return nil, fmt.Errorf("escher: expected one SpContainer record")
	}
	return parseSpContainer(records[0])
}

// ShapeNodeFromRecord converts an already-parsed SpContainer record without
// re-encoding it. This is the preferred bridge for Scan results.
func ShapeNodeFromRecord(record Record) (*ShapeNode, error) {
	if record.Header.Type != RecordSpContainer {
		return nil, fmt.Errorf("escher: record 0x%04X is not an SpContainer", record.Header.Type)
	}
	return parseSpContainer(record)
}
