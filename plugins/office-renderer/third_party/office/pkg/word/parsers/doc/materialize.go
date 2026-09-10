package doc

import wordevent "office/pkg/word/event"

// materializer remains an internal alias during migration so existing DOC tests
// and callers keep their shape while the actual event -> IR assembly is owned by
// the shared word/event package.
type materializer = wordevent.Materializer

func newMaterializer(name string) *materializer {
	return wordevent.NewMaterializer(name)
}
