package browserpane

import (
	"strings"
	"testing"

	"wails-app/internal/webresourcecache"
)

func TestHandleBrowserPaneBridgeMessageStoresResourcesAndEmitsUpdate(t *testing.T) {
	const pageURL = "https://example.com/view"
	webresourcecache.Clear(pageURL)
	t.Cleanup(func() {
		webresourcecache.Clear(pageURL)
	})

	var emitted string
	handleBrowserPaneBridgeMessage(`{
		"type":"resource-snapshot",
		"pageUrl":"https://example.com/view#fragment",
		"resources":[
			{"url":"https://example.com/app.js","size":42,"initiatorType":"script"},
			{"url":"https://example.com/view","size":99,"initiatorType":"document"}
		]
	}`, func(path string) {
		emitted = path
	}, nil)

	if emitted != pageURL {
		t.Fatalf("expected emitted path %q, got %q", pageURL, emitted)
	}

	resources := webresourcecache.Load(pageURL)
	if len(resources) != 1 {
		t.Fatalf("expected 1 cached resource, got %d", len(resources))
	}
	if resources[0].URL != "https://example.com/app.js" {
		t.Fatalf("expected cached resource url %q, got %q", "https://example.com/app.js", resources[0].URL)
	}
	if resources[0].Size != 42 {
		t.Fatalf("expected cached resource size 42, got %d", resources[0].Size)
	}
	if resources[0].InitiatorType != "script" {
		t.Fatalf("expected cached initiator type %q, got %q", "script", resources[0].InitiatorType)
	}
}

func TestHandleBrowserPaneBridgeMessageForwardsGenericMessages(t *testing.T) {
	var forwarded string
	handleBrowserPaneBridgeMessage(`{"type":"viewer-command","command":"refresh"}`, nil, func(message string) {
		forwarded = message
	})

	if forwarded != `{"type":"viewer-command","command":"refresh"}` {
		t.Fatalf("expected forwarded message, got %q", forwarded)
	}
}

func TestBrowserPaneInjectedBootstrapScriptSupportsWindowsAndWebKitHosts(t *testing.T) {
	script := browserPaneInjectedBootstrapScript()

	requiredFragments := []string{
		"window.__wailsBrowserPaneBridgeInit",
		"window.chrome&&window.chrome.webview&&window.chrome.webview.postMessage",
		"window.webkit&&window.webkit.messageHandlers&&window.webkit.messageHandlers.wailsBrowserPane&&window.webkit.messageHandlers.wailsBrowserPane.postMessage",
		"bridge.receiveHostMessage=function(raw){",
		"bridge.handleRequest=async function(data)",
		"window.__wailsBrowserPaneFlushResources=schedule",
		"type:'resource-snapshot'",
		"window.__wailsBrowserPaneLifecycleInit",
		"type:'lifecycle'",
		"window.__wailsBrowserPaneShortcutInit",
		"type:'shortcut'",
	}
	for _, fragment := range requiredFragments {
		if !strings.Contains(script, fragment) {
			t.Fatalf("expected script to contain %q", fragment)
		}
	}
}

func TestBrowserPaneInjectedBridgeSupportsRpcOnrequest(t *testing.T) {
	script := browserPaneInjectedMessageBridgeScript()

	requiredFragments := []string{
		"bridge.onrequest=null",
		"bridge.handleRequest=async function(data)",
		"bridge.postMessage({_rpc:true,id:data.id,result:result})",
		"bridge.postMessage({_rpc:true,id:data.id,error:",
		"detail._rpc&&detail.id&&typeof detail.type!=='undefined'&&typeof bridge.onrequest==='function'",
	}
	for _, fragment := range requiredFragments {
		if !strings.Contains(script, fragment) {
			t.Fatalf("expected script to contain %q", fragment)
		}
	}
}

func TestBrowserPaneHostMessageEvalScriptUsesInjectedBridge(t *testing.T) {
	script := browserPaneHostMessageEvalScript(`{"kind":"ping"}`)
	requiredFragments := []string{
		`var bridge=window.__wailsBrowserPane;`,
		`bridge.receiveHostMessage(raw);`,
		`new CustomEvent('wails-browser-pane-message',{detail:detail})`,
	}
	for _, fragment := range requiredFragments {
		if !strings.Contains(script, fragment) {
			t.Fatalf("expected host message script to contain %q", fragment)
		}
	}
}