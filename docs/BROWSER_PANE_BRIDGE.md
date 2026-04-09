# Browser Pane Bridge

## Purpose

`internal/browserpane` hosts a native child webview for a surface and exposes a uniform bidirectional message bridge to Angular and to the page loaded inside that child webview.

The bridge is used for two separate concerns:

- Application messages between Angular and the child page.
- Internal browser-pane messages such as resource snapshots used by the web resource cache.

This document describes the runtime contract, the message flow, and the platform-specific delivery behavior.

## Architecture

The bridge is a relay, not a direct Angular-to-page JavaScript call.

Host to child flow:

`Angular -> Wails binding -> Go browser pane service -> native webview -> child page`

Child to host flow:

`child page -> native webview message handler -> Go browser pane service -> Wails event -> Angular`

The common bridge logic lives in:

- `internal/browserpane/message_bridge.go`
- `internal/browserpane/service.go`
- `frontend/src/app/services/native-browser-pane.service.ts`

Platform adapters live in:

- `internal/browserpane/pane_windows.go`
- `internal/browserpane/pane_linux.go`
- `internal/browserpane/pane_darwin.go`

## Shared Page API

Every platform injects the same bootstrap script into the child page before or during document load. That script installs `window.__wailsBrowserPane`.

The injected bridge provides:

- `window.__wailsBrowserPane.postMessage(payload)`
- `window.__wailsBrowserPane.receiveHostMessage(raw)`
- `window.__wailsBrowserPane.onmessage`
- `window.__wailsBrowserPane.parseMessage(raw)`

It also dispatches a DOM event when the host posts a message into the page:

- `wails-browser-pane-message`

Message serialization rules:

- The native boundary always carries a string.
- Objects are serialized with `JSON.stringify(...)`.
- `undefined` is normalized to the string `"null"`.
- On receipt, the bridge tries `JSON.parse(...)`.
- If parsing fails, the raw string is delivered unchanged.

## Host To Child

Angular sends a message with `NativeBrowserPaneService.postMessage(surfaceId, message)`.

Runtime path:

1. `NativeBrowserPaneService.postMessage(...)` serializes the payload to a string and calls `window.go.app.App.BrowserPanePostMessage(surfaceId, message)`.
2. `App.BrowserPanePostMessage(...)` forwards to `browserpane.Service.PostMessage(...)`.
3. `browserpane.Service` resolves the pane by `surfaceId` and calls the platform pane's `PostMessage(...)`.
4. The platform layer evaluates the shared host-delivery script returned by `browserPaneHostMessageEvalScript(...)`.
5. Inside the child page, the injected bridge dispatches the `wails-browser-pane-message` event and also invokes `window.__wailsBrowserPane.onmessage(...)` when a handler is set.

Angular entry points:

- `frontend/src/app/services/native-browser-pane.service.ts`
- `internal/app/bindings_browserpane.go`
- `internal/browserpane/service.go`

Page-side receive options:

```js
window.addEventListener('wails-browser-pane-message', (event) => {
  console.log('host event detail', event.detail);
});

window.__wailsBrowserPane.onmessage = ({ data, rawData }) => {
  console.log('host message', data, rawData);
};
```

## Child To Host

The child page sends a message with `window.__wailsBrowserPane.postMessage(payload)`.

Runtime path:

1. The injected bridge serializes the payload to a string.
2. The bridge sends that string through the native webview message channel.
3. The platform adapter receives the native message and forwards it into Go.
4. `handleBrowserPaneBridgeMessage(...)` inspects the payload.
5. Internal bridge messages are consumed by Go.
6. All other messages are emitted back to Angular as the Wails event `browser-pane:message`.

Angular subscribes through:

- `NativeBrowserPaneService.onMessage(...)`
- `NativeBrowserPaneService.onSurfaceMessage(surfaceId, ...)`

Angular receives:

- `surfaceId`: assigned by the host, not by the child page
- `message`: the raw string payload
- `data`: parsed JSON when valid, otherwise the raw string

Page-side send example:

```js
window.__wailsBrowserPane?.postMessage({
  type: 'child-ready',
  title: document.title,
  href: location.href,
});
```

Angular-side receive example:

```ts
const stop = nativeBrowserPane.onSurfaceMessage(surfaceId, ({ message, data }) => {
  console.log('raw', message);
  console.log('parsed', data);
});
```

## Reserved Internal Messages

The bridge reserves one message type for host-managed behavior:

- `type: "resource-snapshot"`

When Go receives that message type, it does not forward it as a normal app message. Instead it:

- Normalizes `pageUrl`
- Stores the resource list in `webresourcecache`
- Emits `web:resourcesChanged` when the cache changed

Application code should not reuse `type: "resource-snapshot"` unless it intentionally wants to target this internal cache path.

## Page Ready And Queueing

Host-to-child delivery is queued until the current page is ready to execute injected JavaScript.

Platform behavior:

- Windows queues host messages until WebView2 navigation completes.
- Linux queues scripts until WebKit reports `WEBKIT_LOAD_FINISHED`.
- macOS queues scripts until `WKWebView` reports `didFinishNavigation`.

This means:

- Calling `BrowserPanePostMessage(...)` immediately after navigation is valid.
- The message is not dropped just because the page is still loading.
- Delivery order is preserved within a pane.

Child-to-host messages are only available after the bootstrap script is present in the page.

## Platform Transport

The page uses the native transport available on the active platform.

- Windows: `window.chrome.webview.postMessage(...)`
- Linux WebKitGTK: `window.webkit.messageHandlers.wailsBrowserPane.postMessage(...)`
- macOS WKWebView: `window.webkit.messageHandlers.wailsBrowserPane.postMessage(...)`

The Go side normalizes these platform differences into the same callback path.

## Practical Contract

For application messages, treat the bridge as a JSON-over-string channel with host-assigned `surfaceId`.

Recommended conventions:

- Send JSON objects with an explicit `type`.
- Keep payloads self-contained and serializable.
- Avoid overloading reserved internal message types.
- Subscribe per surface when the same Angular session can host multiple child panes.

Recommended message shape:

```json
{
  "type": "example",
  "payload": {
    "id": "123"
  }
}
```

## Current Limitations

- The bridge is message-based only. There is no request-response correlation, timeout handling, or RPC layer on top of it.
- The native boundary carries strings, so large payloads should be avoided unless there is a clear reason.
- Internal `resource-snapshot` traffic shares the same transport as app messages, even though Go treats it specially.

If the bridge grows beyond simple event-style traffic, the next step should be an explicit protocol layer with:

- message IDs
- request and response types
- versioning
- error envelopes
- optional backpressure limits
