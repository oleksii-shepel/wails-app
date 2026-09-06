package legacy

import "office/pkg/ooxml/cfb"

// WalkOpenedCFB remains only as the migration bridge for encrypted/pre-Word97
// DOC inputs. Normal Word97+ enters the canonical doc/word97 streaming stack.
func WalkOpenedCFB(f *cfb.File, handler StreamHandler) error { return walkDocCFB(f, handler) }
