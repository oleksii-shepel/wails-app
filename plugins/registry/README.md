# Reassembly Registry Plugin

Universal Windows Registry plugin for Reassembly.

This package owns the complete `registry://` feature: frontend presentation and
commands plus the native Windows backend. Manager must not mount a Registry VFS,
register a Registry panel source, or provide Registry-specific operations.

## Layout

- `src/` — Angular Registry UI, editors, panel source and command contributions.
- `src/universal/` — adapter from the universal plugin runtime to the existing UI.
- `frontend/index.js` — archive activation entry.
- `backend/` — Windows-native newline-delimited JSON RPC backend.
- `plugin.json` — one-target universal package manifest (`windows-x64`).

## Backend protocol

The executable reads one JSON request per line from stdin and writes one JSON
response per line to stdout. Supported methods are:

- `hello`
- `registry.list`
- `registry.read`
- `registry.write`
- `registry.createKey`
- `registry.delete`
- `registry.rename`

The backend is started and owned by the universal plugin host. Frontend code
accesses it only through `ctx.backend.call(...)`.

## Build backend

```bash
cd backend
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -H=windowsgui" -o plugin.exe .
```

## Host invariant

With this plugin absent or inactive, `registry://` must not resolve. The Manager
must contain no built-in Registry mount/provider/autocomplete implementation.
