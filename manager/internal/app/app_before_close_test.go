package app

import (
	"context"
	"testing"
)

func TestBeforeCloseNeverPreventsShutdown(t *testing.T) {
	app := &App{}
	if app.BeforeClose(context.Background()) {
		t.Fatal("shutdown should not be blocked by a confirmation dialog")
	}
}
