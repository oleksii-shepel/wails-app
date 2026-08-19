package browserpane

import (
	"strings"
	"testing"
)

func TestHandlePaneMessageEmitsLifecycleEvents(t *testing.T) {
	var eventName string
	var eventData map[string]string
	s := NewService(func(name string, data interface{}) {
		eventName = name
		eventData, _ = data.(map[string]string)
	})

	s.handlePaneMessage("surface-1", `{"type":"lifecycle","state":"completed","url":"https://example.com"}`)

	if eventName != "browser-pane:lifecycle" {
		t.Fatalf("expected event %q, got %q", "browser-pane:lifecycle", eventName)
	}
	if eventData["surfaceId"] != "surface-1" {
		t.Fatalf("expected surfaceId %q, got %q", "surface-1", eventData["surfaceId"])
	}
	if eventData["state"] != "completed" {
		t.Fatalf("expected state %q, got %q", "completed", eventData["state"])
	}
	if eventData["url"] != "https://example.com" {
		t.Fatalf("expected url %q, got %q", "https://example.com", eventData["url"])
	}
}

func TestHandlePaneMessageForwardsGenericMessages(t *testing.T) {
	var eventName string
	var eventData map[string]string
	s := NewService(func(name string, data interface{}) {
		eventName = name
		eventData, _ = data.(map[string]string)
	})

	s.handlePaneMessage("surface-2", `{"type":"viewer-command","command":"refresh"}`)

	if eventName != "browser-pane:message" {
		t.Fatalf("expected event %q, got %q", "browser-pane:message", eventName)
	}
	if eventData["surfaceId"] != "surface-2" {
		t.Fatalf("expected surfaceId %q, got %q", "surface-2", eventData["surfaceId"])
	}
	if eventData["message"] != `{"type":"viewer-command","command":"refresh"}` {
		t.Fatalf("expected message %q, got %q", `{"type":"viewer-command","command":"refresh"}`, eventData["message"])
	}
}

func TestHandlePaneMessageIgnoresEmptyMessages(t *testing.T) {
	var called bool
	s := NewService(func(name string, data interface{}) {
		called = true
	})

	s.handlePaneMessage("surface-3", "")
	s.handlePaneMessage("surface-3", "   ")

	if called {
		t.Fatalf("expected no events for empty messages")
	}
}

func TestSyncEmitsStartedLifecycleEventOnURLChange(t *testing.T) {
	var events []map[string]string
	s := NewService(func(name string, data interface{}) {
		if name == "browser-pane:lifecycle" {
			events = append(events, data.(map[string]string))
		}
	})

	// First sync with a URL should emit started
	err := s.Sync(Update{
		SurfaceID: "surface-a",
		URL:       "https://example.com",
		Visible:   true,
		Width:     100,
		Height:    100,
	})
	if err == nil {
		// pane creation will fail because there's no main window in tests,
		// but the lifecycle event should still be emitted before that
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 lifecycle event, got %d", len(events))
	}
	if events[0]["state"] != "started" {
		t.Fatalf("expected state %q, got %q", "started", events[0]["state"])
	}
}

func TestHandlePaneMessageRoutesRpcResponses(t *testing.T) {
	var eventName string
	var eventData map[string]string
	s := NewService(func(name string, data interface{}) {
		eventName = name
		eventData, _ = data.(map[string]string)
	})

	s.handlePaneMessage("surface-rpc", `{"_rpc":true,"id":"42","result":{"x":1}}`)

	if eventName != "browser-pane:rpc" {
		t.Fatalf("expected event %q, got %q", "browser-pane:rpc", eventName)
	}
	if eventData["surfaceId"] != "surface-rpc" {
		t.Fatalf("expected surfaceId %q, got %q", "surface-rpc", eventData["surfaceId"])
	}
	if !strings.Contains(eventData["message"], `"id":"42"`) {
		t.Fatalf("expected message to contain rpc id, got %q", eventData["message"])
	}
}

func TestHandlePaneMessageRoutesGenericMessagesAfterRpcFilter(t *testing.T) {
	var eventName string
	s := NewService(func(name string, data interface{}) {
		eventName = name
		_, _ = data.(map[string]string)
	})

	// Missing 'id' → not an RPC envelope → forwarded as generic message
	s.handlePaneMessage("surface-3", `{"_rpc":true,"result":123}`)

	if eventName != "browser-pane:message" {
		t.Fatalf("expected event %q, got %q", "browser-pane:message", eventName)
	}
}