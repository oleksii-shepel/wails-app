package legacy

import (
    "testing"

    "office/pkg/officeart/legacy/escher"
)

func TestShapeFromEscherUsesMSODRAWFlipFlagOrdering(t *testing.T) {
    tests := []struct {
        name  string
        flags uint32
        flipH bool
        flipV bool
    }{
        {name: "horizontal", flags: 0x40, flipH: true},
        {name: "vertical", flags: 0x80, flipV: true},
        {name: "both", flags: 0xC0, flipH: true, flipV: true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := ShapeFromEscher(&escher.ShapeNode{Flags: tt.flags})
            if got == nil {
                t.Fatal("ShapeFromEscher returned nil")
            }
            if got.Transform.FlipH != tt.flipH || got.Transform.FlipV != tt.flipV {
                t.Fatalf("flags %#x => flipH=%t flipV=%t, want %t/%t", tt.flags, got.Transform.FlipH, got.Transform.FlipV, tt.flipH, tt.flipV)
            }
        })
    }
}
