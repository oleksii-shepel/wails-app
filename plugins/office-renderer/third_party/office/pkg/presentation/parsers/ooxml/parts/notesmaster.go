package parts

// DefaultNotesMaster returns a minimal notes master containing the required
// placeholders so that notes slides can be written back to a PPTX package.
func DefaultNotesMaster() *NotesMaster {
	return &NotesMaster{
		CSld: CommonSlideData{
			SpTree: ShapeTree{
				Shapes: []Shape{
					{
						NvSpPr: NonVisualShapeProps{
							CNvPr: CnvPr{ID: 2, Name: "Slide Image Placeholder 1"},
							NvPr:  PlaceholderPr{Ph: &PlaceholderInfo{Type: "sldImg"}},
						},
						SpPr: ShapeProperties{
							Xfrm: &Transform2D{
								Off: Offset{X: 685800, Y: 685800},
								Ext: Extents{CX: 6096000, CY: 3429000},
							},
						},
					},
					{
						NvSpPr: NonVisualShapeProps{
							CNvPr: CnvPr{ID: 3, Name: "Notes Text Placeholder 2"},
							NvPr:  PlaceholderPr{Ph: &PlaceholderInfo{Type: "body", Idx: 1}},
						},
						SpPr: ShapeProperties{
							Xfrm: &Transform2D{
								Off: Offset{X: 685800, Y: 4343400},
								Ext: Extents{CX: 6096000, CY: 4114800},
							},
						},
						TxBody: &TextBody{
							BodyPr: &DMLBodyPr{},
							Paragraphs: []DMLParagraph{
								{EndParaRPr: &DMLRunPr{}},
							},
						},
					},
					{
						NvSpPr: NonVisualShapeProps{
							CNvPr: CnvPr{ID: 4, Name: "Header Placeholder 3"},
							NvPr:  PlaceholderPr{Ph: &PlaceholderInfo{Type: "hdr"}},
						},
						SpPr: ShapeProperties{
							Xfrm: &Transform2D{
								Off: Offset{X: 0, Y: 0},
								Ext: Extents{CX: 2971800, CY: 457200},
							},
						},
						TxBody: &TextBody{
							BodyPr: &DMLBodyPr{},
							Paragraphs: []DMLParagraph{{}},
						},
					},
					{
						NvSpPr: NonVisualShapeProps{
							CNvPr: CnvPr{ID: 5, Name: "Footer Placeholder 4"},
							NvPr:  PlaceholderPr{Ph: &PlaceholderInfo{Type: "ftr"}},
						},
						SpPr: ShapeProperties{
							Xfrm: &Transform2D{
								Off: Offset{X: 0, Y: 6426200},
								Ext: Extents{CX: 2971800, CY: 457200},
							},
						},
						TxBody: &TextBody{
							BodyPr: &DMLBodyPr{},
							Paragraphs: []DMLParagraph{{}},
						},
					},
					{
						NvSpPr: NonVisualShapeProps{
							CNvPr: CnvPr{ID: 6, Name: "Slide Number Placeholder 5"},
							NvPr:  PlaceholderPr{Ph: &PlaceholderInfo{Type: "sldNum"}},
						},
						SpPr: ShapeProperties{
							Xfrm: &Transform2D{
								Off: Offset{X: 3886200, Y: 6426200},
								Ext: Extents{CX: 2971800, CY: 457200},
							},
						},
						TxBody: &TextBody{
							BodyPr: &DMLBodyPr{},
							Paragraphs: []DMLParagraph{{}},
						},
					},
					{
						NvSpPr: NonVisualShapeProps{
							CNvPr: CnvPr{ID: 7, Name: "Date Placeholder 6"},
							NvPr:  PlaceholderPr{Ph: &PlaceholderInfo{Type: "dt", Idx: 1}},
						},
						SpPr: ShapeProperties{
							Xfrm: &Transform2D{
								Off: Offset{X: 3886200, Y: 0},
								Ext: Extents{CX: 2971800, CY: 457200},
							},
						},
						TxBody: &TextBody{
							BodyPr: &DMLBodyPr{},
							Paragraphs: []DMLParagraph{{}},
						},
					},
				},
				Items: []ShapeTreeItem{
					{Kind: ShapeTreeItemShape, Index: 0},
					{Kind: ShapeTreeItemShape, Index: 1},
					{Kind: ShapeTreeItemShape, Index: 2},
					{Kind: ShapeTreeItemShape, Index: 3},
					{Kind: ShapeTreeItemShape, Index: 4},
					{Kind: ShapeTreeItemShape, Index: 5},
					{Kind: ShapeTreeItemShape, Index: 6},
				},
			},
		},
	}
}