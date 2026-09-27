package legacy

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	officeartir "office/pkg/officeart/ir"
	officeartlegacy "office/pkg/officeart/legacy"
	officeartescher "office/pkg/officeart/legacy/escher"
	"office/pkg/ooxml/cfb"
)

func TestPass9OwnershipAuditTreatsUndecodedNoIDContainerAsStructural(t *testing.T) {
	scene := legacyPptVisualScene{}
	scene.dispatch.containers.containers = []legacyPptShapeContainer{{}}
	audit := scene.ownershipAudit(nil)
	if len(audit) != 1 {
		t.Fatalf("ownership entries = %d, want 1", len(audit))
	}
	if !audit[0].Claimed || audit[0].Reason != "structural: no authored ShapeID" {
		t.Fatalf("no-ID structural container = %+v", audit[0])
	}
}

// TestPass9LegacyPptCorpusOwnershipClosure is the corpus gate for the takeover.
// It is opt-in because the legacy fixture directory is intentionally not part of
// small cumulative parser overlays. Run in the full repository with:
//
//	LEGACY_PPT_CORPUS_AUDIT=1 go test ./pkg/presentation/parsers/legacy \
//	    -run TestPass9LegacyPptCorpusOwnershipClosure -count=1 -v
//
// Every authored SpContainer with a recoverable ShapeID must either reach
// canonical IR or have an intentional structural/background owner. Failures are
// grouped by ownership reason so the next fix closes a category, not a fixture.
func TestPass9LegacyPptCorpusOwnershipClosure(t *testing.T) {
	if os.Getenv("LEGACY_PPT_CORPUS_AUDIT") == "" {
		t.Skip("set LEGACY_PPT_CORPUS_AUDIT=1 to audit the full PPT corpus")
	}
	paths, err := filepath.Glob("../../../../testdata/ppt/*.ppt")
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("no testdata/ppt/*.ppt fixtures found")
	}
	sort.Strings(paths)

	type gap struct {
		file, owner, reason, category string
		persistID                     int
		shapeID                       uint32
		shapeType                     uint16
		flags                         uint32
		groupDepth                    int
		clientAnchorBytes             int
		childAnchorBytes              int
		lineLike                      bool
		tableMember                   bool
		hasClientTextbox              bool
		anchorDetail                  string
		visualDetail                  string
		tableDetail                   string
	}
	var gaps []gap
	auditedScenes := 0

	auditScene := func(file, owner string, persistID int, scene legacyPptVisualScene) {
		if scene.empty() {
			return
		}
		auditedScenes++
		projection := scene.completeProjection(legacySlideText{PersistID: persistID}, nil, nil, nil, nil, nil, nil)
		containersByID := make(map[uint32]legacyPptShapeContainer)
		for _, container := range scene.dispatch.containers.shapeContainers() {
			if container.ShapeID != 0 {
				containersByID[container.ShapeID] = container
			}
		}
		tableGroupPaths := make([][]int, 0)
		for _, group := range scene.dispatch.containers.shapeGroups() {
			if legacyPptGroupContainerIsTable(group.Event.Payload) {
				tableGroupPaths = append(tableGroupPaths, group.Path)
			}
		}
		for _, entry := range projection.unclaimed() {
			g := gap{file: file, owner: owner, persistID: persistID, shapeID: entry.ShapeID, reason: entry.Reason}
			if container, ok := containersByID[entry.ShapeID]; ok {
				g.groupDepth = container.GroupDepth
				for _, tablePath := range tableGroupPaths {
					if legacyPptGroupPathHasPrefix(container.GroupPath, tablePath) {
						g.tableMember = true
						break
					}
				}
				if node := container.Node; node != nil {
					g.shapeType = node.ShapeType
					g.flags = node.Flags
					g.clientAnchorBytes = len(node.ClientAnchor)
					g.childAnchorBytes = len(node.ChildAnchor)
					g.lineLike = legacyPptConnectorType(node.ShapeType) != ""
					g.hasClientTextbox = node.HasClientTextbox
					g.anchorDetail = pass9LegacyAnchorDetail(node.ClientAnchor, node.ChildAnchor)
					g.visualDetail = pass9LegacyVisualDetail(node)
				}
			}
			switch {
			case g.tableMember && g.lineLike:
				g.category = "table-line"
			case g.tableMember:
				g.category = "table-member"
			case g.lineLike:
				g.category = "line-like"
			case g.hasClientTextbox:
				g.category = "textbox"
			default:
				g.category = fmt.Sprintf("shapeType-%d", g.shapeType)
			}
			if strings.Contains(g.reason, "explicit table group did not materialize") {
				g.tableDetail = pass9LegacyTableFailureDetail(scene, g.shapeID)
			}
			gaps = append(gaps, g)
		}
	}

	for _, path := range paths {
		f, err := cfb.Open(path)
		if err != nil {
			// Pre-97/non-CFB fixtures belong to their own compatibility parser and
			// have no PPT97 OfficeArt SpContainer ownership contract to audit.
			continue
		}
		stream, ok := f.Stream("PowerPoint Document")
		if !ok {
			continue
		}
		decrypted, _, err := maybeDecryptPptStream(f, stream)
		if err != nil {
			// Password-protected fixtures are covered by dedicated crypto tests;
			// without a plaintext OfficeArt stream there is nothing to classify.
			continue
		}
		stream = decrypted
		ctx, ok := extractPptPersistContext(f, stream)
		if !ok {
			continue
		}
		events := collectLegacyPptPersistEventIndex(stream, ctx)
		base := filepath.Base(path)
		for _, persistID := range ctx.slidePersistIDs {
			if event, ok := events.slides[persistID]; ok {
				auditScene(base, "slide", persistID, collectLegacyPptVisualScene(event.Payload))
			}
		}
		for _, entry := range ctx.masterEntries {
			if event, ok := events.masters[entry.RefID]; ok {
				auditScene(base, "master", entry.RefID, collectLegacyPptVisualScene(event.Payload))
			}
		}
	}

	if auditedScenes == 0 {
		t.Fatal("no PPT97 OfficeArt scenes were audited")
	}
	if len(gaps) == 0 {
		t.Logf("Pass 9 ownership closure: %d scenes, zero unexplained SpContainers", auditedScenes)
		return
	}

	counts := map[string]int{}
	for _, g := range gaps {
		counts[g.reason+" | "+g.category]++
	}
	reasons := make([]string, 0, len(counts))
	for reason := range counts {
		reasons = append(reasons, reason)
	}
	sort.Strings(reasons)
	var summary []string
	for _, reason := range reasons {
		summary = append(summary, fmt.Sprintf("%s=%d", reason, counts[reason]))
	}
	for _, g := range gaps {
		t.Logf("UNCLAIMED file=%s owner=%s persist=%d shapeID=%d reason=%s category=%s shapeType=%d flags=0x%08x groupDepth=%d tableMember=%t lineLike=%t textbox=%t clientAnchorBytes=%d childAnchorBytes=%d anchors=%s visual=%s %s",
			g.file, g.owner, g.persistID, g.shapeID, g.reason, g.category, g.shapeType, g.flags, g.groupDepth, g.tableMember, g.lineLike, g.hasClientTextbox, g.clientAnchorBytes, g.childAnchorBytes, g.anchorDetail, g.visualDetail, g.tableDetail)
	}
	t.Fatalf("Pass 9 ownership closure has %d unexplained SpContainers across %d scenes: %s", len(gaps), auditedScenes, strings.Join(summary, ", "))
}

func pass9LegacyAnchorDetail(clientAnchor, childAnchor []byte) string {
	var parts []string
	switch {
	case len(clientAnchor) >= 16:
		parts = append(parts, fmt.Sprintf("client(top=%d,left=%d,right=%d,bottom=%d)",
			int32(binary.LittleEndian.Uint32(clientAnchor[0:4])),
			int32(binary.LittleEndian.Uint32(clientAnchor[4:8])),
			int32(binary.LittleEndian.Uint32(clientAnchor[8:12])),
			int32(binary.LittleEndian.Uint32(clientAnchor[12:16]))))
	case len(clientAnchor) >= 8:
		parts = append(parts, fmt.Sprintf("client(top=%d,left=%d,right=%d,bottom=%d)",
			int16(binary.LittleEndian.Uint16(clientAnchor[0:2])),
			int16(binary.LittleEndian.Uint16(clientAnchor[2:4])),
			int16(binary.LittleEndian.Uint16(clientAnchor[4:6])),
			int16(binary.LittleEndian.Uint16(clientAnchor[6:8]))))
	}
	if len(childAnchor) >= 16 {
		parts = append(parts, fmt.Sprintf("child(left=%d,top=%d,right=%d,bottom=%d)",
			int32(binary.LittleEndian.Uint32(childAnchor[0:4])),
			int32(binary.LittleEndian.Uint32(childAnchor[4:8])),
			int32(binary.LittleEndian.Uint32(childAnchor[8:12])),
			int32(binary.LittleEndian.Uint32(childAnchor[12:16]))))
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, ";")
}

func pass9LegacyVisualDetail(node *officeartescher.ShapeNode) string {
	if node == nil {
		return "nil"
	}
	shared := officeartlegacy.ShapeFromEscher(node)
	if shared == nil {
		return "none"
	}
	var parts []string
	if shared.Fill.Kind != officeartir.FillUnset && shared.Fill.Kind != officeartir.FillNone {
		parts = append(parts, "fill="+string(shared.Fill.Kind))
	}
	if shared.Line.Visible {
		parts = append(parts, "line")
	}
	if shared.Geometry.Kind == officeartir.GeometryCustom && len(shared.Geometry.Paths) != 0 {
		parts = append(parts, fmt.Sprintf("customPaths=%d", len(shared.Geometry.Paths)))
	}
	if shared.GeometryText != nil && shared.GeometryText.Text != "" {
		parts = append(parts, "geometryText")
	}
	if shared.Image != nil {
		parts = append(parts, "image")
	}
	if node.HasClientTextbox {
		parts = append(parts, fmt.Sprintf("textboxBytes=%d", len(node.ClientTextbox)))
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, "+")
}

func pass9LegacyTableFailureDetail(scene legacyPptVisualScene, rootID uint32) string {
	for _, group := range scene.dispatch.containers.shapeGroups() {
		id, ok := legacyPptGroupRootShapeID(group.Event.Payload)
		if !ok || id != rootID {
			continue
		}
		descendants := 0
		textboxes := 0
		placeableTextboxes := 0
		gridLines := 0
		placeableGridLines := 0
		for _, container := range scene.dispatch.containers.containersUnder(group.Path) {
			if legacyPptSceneContainerOwnedByNestedExplicitTable(scene.dispatch.containers, group.Path, container.GroupPath) {
				continue
			}
			descendants++
			if container.Node == nil {
				continue
			}
			placement := resolveLegacyPptShapePlacement(container.Node, scene.frameForContainer(container))
			if container.Node.ShapeType == 20 {
				gridLines++
				if placement.valid() && (placement.Bounds.W > 0 || placement.Bounds.H > 0) {
					placeableGridLines++
				}
			}
			if container.Node.HasClientTextbox {
				textboxes++
				if placement.valid() && placement.Bounds.W > 0 && placement.Bounds.H > 0 {
					placeableTextboxes++
				}
			}
		}
		_, sceneOK := parseLegacyPptTableShapeFromSceneGroupForSlide(
			scene.dispatch.containers, group, legacySlideText{}, nil, nil, nil, 1,
		)
		_, recoveryOK := recoverLegacyPptTableShapeFromGroupContainer(
			group.Event.Payload, group.ParentFrame, nil, nil, nil, 1,
		)
		return fmt.Sprintf("tableDiag(external=%t,desc=%d,textboxes=%d,placeableTextboxes=%d,grid=%d,placeableGrid=%d,scene=%t,recovery=%t)",
			legacyPptOfficeArtContainsExternalObject(group.Event.Payload),
			descendants, textboxes, placeableTextboxes, gridLines, placeableGridLines, sceneOK, recoveryOK)
	}
	return "tableDiag(group-not-found)"
}
