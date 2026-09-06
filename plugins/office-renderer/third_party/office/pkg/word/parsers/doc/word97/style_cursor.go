package word97

import (
	"strings"
	wordir "office/pkg/word/ir"
)

type StyleResolver func(int) (wordir.ParagraphFormat, bool)
type StyleApplicator func(*Run, wordir.ParagraphFormat)

type styleCursor struct { inner Cursor; resolve StyleResolver; apply StyleApplicator }

func NewStyleCursor(inner Cursor, resolve StyleResolver, apply StyleApplicator) Cursor { if inner==nil{return nil};if resolve==nil{return inner};if apply==nil{apply=ApplyParagraphStyle};return &styleCursor{inner:inner,resolve:resolve,apply:apply} }

func (c *styleCursor) Next()(Run,bool,error){
	run,ok,err:=c.inner.Next();if err!=nil||!ok{return run,ok,err};if run.ParagraphFormat.BinaryStyleIndex==nil{return run,true,nil};styleFormat,exists:=c.resolve(*run.ParagraphFormat.BinaryStyleIndex);if !exists{return run,true,nil};c.apply(&run,styleFormat)
	if run.ParagraphFormat.BinaryListOverride==nil&&styleFormat.BinaryListOverride!=nil{v:=*styleFormat.BinaryListOverride;run.ParagraphFormat.BinaryListOverride=&v};if run.ParagraphFormat.BinaryListLevel==nil&&styleFormat.BinaryListLevel!=nil{v:=*styleFormat.BinaryListLevel;run.ParagraphFormat.BinaryListLevel=&v};if run.ParagraphFormat.BinaryListFormat==nil&&styleFormat.BinaryListFormat!=nil{v:=*styleFormat.BinaryListFormat;run.ParagraphFormat.BinaryListFormat=&v}
	if run.ParagraphFormat.InTable||styleFormat.InTable{run.ParagraphFormat.InTable=true;run.LegacyInTableHint=true;if strings.HasSuffix(run.Text,"\a"){run.ParagraphFormat.IsTableCellTrailingParagraph=true}}
	return run,true,nil
}
