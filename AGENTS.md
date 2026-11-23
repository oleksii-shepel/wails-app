# AGENTS.md

Guidance for AI coding agents working in this repository. Read this before making changes.

## Project

Reassembly ("Aether") — a Wails desktop file manager: Go backend + Angular frontend. Dual-pane browsing, multiple views (list/grid/details/tree), archive browsing and mutation, batch operations, workspace persistence, Windows elevation fallbacks, and a plugin system (compile-time workspace plugins plus user-installed declarative directory plugins).

Primary dev and target platform is Windows. CI (`.github/workflows/build.yml`) runs on Ubuntu: `npm run check` in `manager/frontend` and `go test ./...` in `manager/`.

## Workspace layout

```text
manager/                  Wails host app
  frontend/               Angular 22 app (src/app: components, services, surfaces, viewers, plugin)
  internal/               Backend services (app, archive, batch, navigation, workspace,
                         elevation, runtime, platform, plugin, pluginpackage, ...)
  vfs/                    Virtual filesystem providers (incl. registry backend, memory sidecar)
  third_party/            Vendored deps (go-webview2); excluded from lint
packages/core/            @reassembly/core (TS contracts/runtime) + Go module
                         github.com/epikodelabs/reassembly/core — shared by host and plugins
plugins/
  registry/               Built-in workspace plugin (@reassembly/registry-plugin)
  office-renderer/        Office document renderer plugin (backend + vendored third_party/office)
docs/                     Feature/architecture docs; see docs/README.md index
scripts/                  Dev wrappers and verification/ownership audits
go.work                   Joins ./manager and ./packages/core/go only
```

## Commands

Run npm commands from the repo root.

To start the app: run `npm install` once from the repo root, then `wails dev` from `manager/`.

| Task | Command |
|---|---|
| Live dev (Windows) | `npm ci` at the repo root, then `npm run dev:app` |
| Windows desktop build | `npm run build:app` |
| Frontend check (CI parity) | `npm --prefix manager/frontend run check` (testify tests + production build) |
| Go tests (CI parity) | `cd manager && go test ./...` |
| Registry ownership audit | `node scripts/registry-boundary.mjs` (`--fix` removes obsolete files) |
| Theme color audit | `cd manager/frontend && npm run audit:theme` |

### Build order matters

`manager/frontend` resolves `@reassembly/core` and `@reassembly/registry-plugin` from local source directories. Run npm installs from the repository root. The desktop build embeds the compiled Angular assets, so build the frontend before running Go checks that include the host package.

Plugin backends are standalone Go modules (not in `go.work`); build them with each plugin's `npm run build:backend` from the plugin directory. The office-renderer backend uses its vendored `third_party/office` module via a local `replace`.

## Architecture boundaries (enforced rules)

1. **Core stays neutral.** `packages/core` holds cross-language contracts and framework-neutral runtime only. It must not import Manager code, Angular components, Wails bindings, workspace persistence, or concrete provider implementations. Frontend imports use `@reassembly/core/contracts|runtime/...`.
2. **Plugins use public contracts only.** Plugins must not import from `manager/frontend/src/app/components`, private services, or backend implementation packages. Host capabilities are consumed via the published plugin SDK. `manager` itself has no Office dependency — the office-renderer vendors its parser in `plugins/office-renderer/third_party/office`.
3. **Registry ownership is split.** Manager owns the pane shell (`manager/frontend/src/app/components/registry-pane.component.ts`, a Directory-pane clone). The plugin owns the `registry://` panel source, toolbar/context-menu contributions, editors, and command behavior. The Registry backend stays in `manager/vfs/registry` until a backend plugin ABI exists. `scripts/registry-boundary.mjs` audits this split — run it after touching either side.
4. **Manager backend lives in `internal/`.** `App` is only the Wails binding wrapper; orchestration belongs in `internal/{archive,batch,navigation,workspace,...}` services. Don't grow `app.go`.
5. **Manager frontend stays thin at the pane host.** `pane-host.component.ts` owns composition and event adaptation; reusable orchestration goes in controllers, factories, or surface VMs (`src/app/surfaces`, `src/app/viewers`). Prefer grouping browser-facing actions behind pane command objects over adding single-purpose public pane methods.
6. **View operation parity.** List, grid, details, and tree are equivalent interaction surfaces. A new operation must be wired through `pane-view-host.component.ts` and the pane shell for all four views, and the parity matrix in `docs/frontend/README.md` updated.
7. **Directory plugins are declarative.** User-installed plugins under the app config directory expose `plugin.json` (+ optional `frontend.json`) as data only. The host never imports/evaluates their JavaScript. Executable-plugin capabilities are a separate design — don't add ad-hoc code execution paths.

## Conventions

- **Go**: go 1.25, module `wails-app` for the manager. Lint per `manager/.golangci.yml` (gofmt, staticcheck, gosec, ...; `third_party` skipped). `unsafe.Pointer`/COM interop and variable-argument subprocess launches are accepted patterns on Windows (gocritic disabled, specific gosec rules excluded) — don't "fix" them.
- **TypeScript/Angular**: Angular 22, TypeScript 6, strict domain-organized code. Tests use `@epikodelabs/testify` with `*.spec.ts` files (`manager/frontend/testify.json`); `npm test` in `manager/frontend` proxies to the root-level testify run.
- **Generated output — never hand-edit**: `dist/`, `artifacts/`, `node_modules/`, `out-tsc/`, `plugin.exe` binaries, `manager/frontend/wailsjs/` (Wails bindings), `manager/frontend/package.json.md5`.
- **Theme**: UI colors come from the theme color system (`docs/THEME_COLOR_SYSTEM.md`), not hardcoded values; `audit:theme` checks this.
- **Docs**: non-trivial features get a doc under `docs/` (see `docs/README.md` for the index) — consult the relevant doc before changing its area, and update it alongside the change.
- **Commits**: short imperative one-liners (see `git log`).
