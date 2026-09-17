package legacy

import (
    "encoding/binary"
    "testing"

    officeartescher "office/pkg/officeart/legacy/escher"
)

func TestLegacyPptSceneTablePartitionExcludesNestedExplicitTableFromAncestor(t *testing.T) {
    tableProp := make([]byte, 6)
    binary.LittleEndian.PutUint16(tableProp[0:2], legacyOfficeArtPropTableProperties)
    binary.LittleEndian.PutUint32(tableProp[2:6], 1)
    nestedRoot := buildOfficeArtRecord(0, legacyOfficeArtSpContainer, 0x000F,
        append(
            append(
                buildOfficeArtRecord(1, legacyOfficeArtSp, 0x0002, officeArtSpPayload(0xA101, 0x01)),
                buildOfficeArtRecord(0, legacyOfficeArtSpgr, 0, classLegacyRect32(0, 0, 500, 500))...,
            ),
            buildOfficeArtRecord(1, officeartescher.RecordTertiaryFOPT, 0x0003, tableProp)...,
        ),
    )
    scene := legacyPptShapeContainerScene{groups: []legacyPptShapeGroup{
        {Path: []int{1}},
        {Path: []int{1, 2}, ParentPath: []int{1}, Event: OfficeArtEvent{Payload: nestedRoot}},
    }}
    if !legacyPptSceneContainerOwnedByNestedExplicitTable(scene, []int{1}, []int{1, 2}) {
        t.Fatal("nested explicit table subtree must be excluded from ancestor table ownership")
    }
    if legacyPptSceneContainerOwnedByNestedExplicitTable(scene, []int{1, 2}, []int{1, 2}) {
        t.Fatal("nested table group must retain ownership of its own descendants")
    }
}
