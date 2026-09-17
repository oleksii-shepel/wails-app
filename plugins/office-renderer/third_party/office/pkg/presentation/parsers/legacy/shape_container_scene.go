package legacy

import officeartescher "office/pkg/officeart/legacy/escher"

// legacyPptShapeContainer is the format-neutral semantic identity of one
// OfficeArt SpContainer inside a drawing. It is materialized once from the
// forward event stream and keeps its group ancestry so downstream projections
// no longer have to rediscover structural ownership independently.
type legacyPptShapeContainer struct {
	Event       OfficeArtEvent
	ShapeID     uint32
	GroupDepth  int
	GroupPath   []int
	IsGroupRoot bool
	Node        *officeartescher.ShapeNode
}

// legacyPptShapeContainerScene is the single structural interpretation of an
// OfficeArt drawing. Family projections still own their mature property/text
// decoding, but consume this shared structural event model.
type legacyPptShapeGroup struct {
	Event       OfficeArtEvent
	Depth       int
	Path        []int
	ParentPath  []int
	ParentFrame legacyPptGroupFrame
	Frame       legacyPptGroupFrame
}

type legacyPptShapeContainerScene struct {
	events     []OfficeArtEvent
	containers []legacyPptShapeContainer
	groups     []legacyPptShapeGroup
}

func decodeLegacyPptShapeContainerScene(events []OfficeArtEvent) legacyPptShapeContainerScene {
	scene := legacyPptShapeContainerScene{events: events}
	if len(events) == 0 {
		return scene
	}
	var groupPath []int
	nextGroup := 0
	frames := map[string]legacyPptGroupFrame{"": {}}
	for _, event := range events {
		switch {
		case event.GroupStart:
			nextGroup++
			parentPath := append([]int(nil), groupPath...)
			parentFrame := frames[legacyPptGroupPathKey(parentPath)]
			groupPath = append(groupPath, nextGroup)
			frame := parentFrame
			if decoded, ok := parseLegacyPptGroupFrameFromSpgrContainer(event.Payload, parentFrame); ok {
				frame = decoded
			}
			frames[legacyPptGroupPathKey(groupPath)] = frame
			scene.groups = append(scene.groups, legacyPptShapeGroup{
				Event: event, Depth: len(groupPath), Path: append([]int(nil), groupPath...), ParentPath: parentPath, ParentFrame: parentFrame, Frame: frame,
			})
		case event.ShapeStart:
			shapeID := uint32(0)
			if event.ShapeNode != nil {
				shapeID = event.ShapeNode.ShapeID
			}
			path := append([]int(nil), groupPath...)
			scene.containers = append(scene.containers, legacyPptShapeContainer{
				Event: event, ShapeID: shapeID, GroupDepth: len(path), GroupPath: path, Node: event.ShapeNode,
				IsGroupRoot: event.ShapeNode != nil && event.ShapeNode.GroupRect != nil,
			})
		case event.GroupEnd:
			if len(groupPath) != 0 {
				groupPath = groupPath[:len(groupPath)-1]
			}
		}
	}
	return scene
}

func (s legacyPptShapeContainerScene) officeArtEvents() []OfficeArtEvent { return s.events }
func (s legacyPptShapeContainerScene) shapeContainers() []legacyPptShapeContainer {
	return s.containers
}
func (s legacyPptShapeContainerScene) shapeGroups() []legacyPptShapeGroup { return s.groups }

func mergeLegacyPptShapeContainerScenes(dst, src legacyPptShapeContainerScene) legacyPptShapeContainerScene {
	if len(src.events) == 0 {
		return dst
	}
	// Group paths are drawing-local. Give every appended drawing a distinct root
	// namespace so equal local group ordinals cannot alias across PPDrawing records.
	groupBase := 0
	for _, c := range dst.containers {
		if len(c.GroupPath) != 0 && c.GroupPath[0] > groupBase {
			groupBase = c.GroupPath[0]
		}
	}
	for _, g := range src.groups {
		clone := g
		clone.Path = append([]int(nil), g.Path...)
		clone.ParentPath = append([]int(nil), g.ParentPath...)
		for i := range clone.Path {
			clone.Path[i] += groupBase
		}
		for i := range clone.ParentPath {
			clone.ParentPath[i] += groupBase
		}
		dst.groups = append(dst.groups, clone)
	}
	for _, c := range src.containers {
		clone := c
		clone.GroupPath = append([]int(nil), c.GroupPath...)
		for i := range clone.GroupPath {
			clone.GroupPath[i] += groupBase
		}
		dst.containers = append(dst.containers, clone)
	}
	dst.events = append(dst.events, src.events...)
	return dst
}

// containersUnder returns the authored non-root shape containers owned by a group.
// Structural recursion is resolved once by the scene; semantic consumers no longer
// walk nested SpgrContainer payloads to rediscover membership.
func (s legacyPptShapeContainerScene) containersUnder(path []int) []legacyPptShapeContainer {
	var out []legacyPptShapeContainer
	for _, c := range s.containers {
		if c.IsGroupRoot || !legacyPptGroupPathHasPrefix(c.GroupPath, path) {
			continue
		}
		out = append(out, c)
	}
	return out
}

// containersDirectlyUnder returns only shapes authored directly in path. Table
// ownership is local to one SpgrContainer: descendants belong to nested groups
// and must be classified by those groups rather than being folded into an
// ancestor table candidate.
func (s legacyPptShapeContainerScene) containersDirectlyUnder(path []int) []legacyPptShapeContainer {
	var out []legacyPptShapeContainer
	for _, c := range s.containers {
		if c.IsGroupRoot || !legacyPptGroupPathEqual(c.GroupPath, path) {
			continue
		}
		out = append(out, c)
	}
	return out
}

func legacyPptGroupPathHasPrefix(path, prefix []int) bool {
	if len(prefix) > len(path) {
		return false
	}
	for i := range prefix {
		if path[i] != prefix[i] {
			return false
		}
	}
	return true
}
