package resolve

import ir "office/pkg/word/ir"

// The following type aliases re-export all IR types from office/pkg/word/ir.
// Existing callers can continue to use resolve.Document, resolve.Style, etc.
// without importing word/ir directly. New format parsers (legacy, ODF) should
// import word/ir directly and produce ir.Document values.
type (
	Document                    = ir.Document
	Story                       = ir.Story
	StoryField                  = ir.StoryField
	Table                       = ir.Table
	TableFormat                 = ir.TableFormat
	TableFloatProps             = ir.TableFloatProps
	TableBorders                = ir.TableBorders
	BorderSide                  = ir.BorderSide
	TableRow                    = ir.TableRow
	TableRowFormat              = ir.TableRowFormat
	TableCell                   = ir.TableCell
	TableCellFormat             = ir.TableCellFormat
	CellBorders                 = ir.CellBorders
	Paragraph                   = ir.Paragraph
	FrameProps                  = ir.FrameProps
	Style                       = ir.Style
	NumberingReference          = ir.NumberingReference
	CharacterStyle              = ir.CharacterStyle
	Numbering                   = ir.Numbering
	ListMarker                  = ir.ListMarker
	Run                         = ir.Run
	TextBox                     = ir.TextBox
	Image                       = ir.Image
	Shape                       = ir.Shape
	ParagraphFormat             = ir.ParagraphFormat
	ParagraphBorders            = ir.ParagraphBorders
	Block                       = ir.Block
	RunFormat                   = ir.RunFormat
	FontFamilyInfo              = ir.FontFamilyInfo
	LatentStyles                = ir.LatentStyles
	LatentStyle                 = ir.LatentStyle
	EmbeddedFont                = ir.EmbeddedFont
	EmbeddedFontFace            = ir.EmbeddedFontFace
	RevisionMark                = ir.RevisionMark
	MoveRange                   = ir.MoveRange
	PermRange                   = ir.PermRange
	Section                     = ir.Section
	LineNumbering               = ir.LineNumbering
	Reference                   = ir.Reference
	PageSize                    = ir.PageSize
	PageMargin                  = ir.PageMargin
	Columns                     = ir.Columns
	DocGrid                     = ir.DocGrid
	Cell3D                      = ir.Cell3D
	TextPath                    = ir.TextPath
	ConditionalTableStyle       = ir.ConditionalTableStyle
	ContentControl              = ir.ContentControl
	SdtListItem                 = ir.SdtListItem
	CustomXmlInfo               = ir.CustomXmlInfo
	CustomXmlAttr               = ir.CustomXmlAttr
	SmartTagInfo                = ir.SmartTagInfo
	FormField                   = ir.FormField
	RubyContent                 = ir.RubyContent
	Field                       = ir.Field
	SignatureLine               = ir.SignatureLine
	TabStop                     = ir.TabStop
	RunBorders                  = ir.RunBorders
	FELayout                    = ir.FELayout
	PageBorders                 = ir.PageBorders
	DocCompatibility            = ir.DocCompatibility
	DocumentProtection          = ir.DocumentProtection
	WriteProtection             = ir.WriteProtection
	DigitalSignature            = ir.DigitalSignature
	DigitalSignatureReference   = ir.DigitalSignatureReference
	DigitalSignatureCertificate = ir.DigitalSignatureCertificate
	ThemeFontLang               = ir.ThemeFontLang
	Theme                       = ir.Theme
	ThemeFontScheme             = ir.ThemeFontScheme
	ThemeFontSet                = ir.ThemeFontSet
	ThemeFormatScheme           = ir.ThemeFormatScheme
	ProofState                  = ir.ProofState
	TableLook                   = ir.TableLook
	TableRowOverride            = ir.TableRowOverride
	AltChunk                    = ir.AltChunk
	InlineMoveRange             = ir.InlineMoveRange
	MailMergeSettings           = ir.MailMergeSettings
	ODSOSettings                = ir.ODSOSettings
	FieldMapData                = ir.FieldMapData
	RecipientData               = ir.RecipientData
	MiscSettings                = ir.MiscSettings
	ReadModeInkLockDown         = ir.ReadModeInkLockDown
	Caption                     = ir.Caption
	AutoCaption                 = ir.AutoCaption
	Captions                    = ir.Captions
	SmartTagType                = ir.SmartTagType
	PaperSource                 = ir.PaperSource
	ActiveWritingStyle          = ir.ActiveWritingStyle
)