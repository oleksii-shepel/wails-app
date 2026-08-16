# Theme Color System

The UI uses semantic VS Code-style tokens from `frontend/src/vscode-theme.scss`. Components should express **intent**, not theme-specific colors.

## Control tokens

- `--vsc-button-background` / `--vsc-button-foreground`
- `--vsc-button-hover`
- `--vsc-button-secondary` / `--vsc-button-secondary-foreground`
- `--vsc-button-disabled-background` / `--vsc-button-disabled-foreground`
- `--vsc-button-danger-background` / `--vsc-button-danger-foreground`
- `--vsc-input-background` / `--vsc-input-foreground`
- `--vsc-focus-ring`

Primary button text must never use `--vsc-foreground-bright`: that token describes text on the current surface, not text on a primary button.

## Surface tokens

- `--vsc-window-background`
- `--vsc-dialog-background` / `--vsc-dialog-foreground`
- `--vsc-menu-background` / `--vsc-menu-foreground`
- `--vsc-popover-background`
- `--vsc-toolbar-background`
- `--vsc-control-background` / `--vsc-control-foreground`
- `--vsc-scrim` / `--vsc-scrim-soft`

Dialogs, menus, overlays and controls should use these instead of dark-only `#1e1e1e`, `#252526`, white translucent hover fills, or literal black scrims.

## Status colors

Use `--vsc-error`, `--vsc-warning`, `--vsc-info`, `--vsc-success`, and the accent tokens. Their light-theme variants are deliberately darker so text remains legible on white/light surfaces.

## Intentional dark surfaces

Media content is an exception. Video stages, image transparency/checkerboard areas, waveform/canvas content and similar viewer media may intentionally remain black or dark when that improves the content itself. The surrounding chrome must still use semantic tokens.

## Contrast targets

The palette is tuned around WCAG-style text contrast as a practical floor:

- normal text: approximately 4.5:1 or better
- large/decorative UI text: approximately 3:1 or better
- primary button: white on `#007acc` is approximately 4.51:1
- light foreground `#3b3b3b` on white is approximately 11.2:1
- light muted foreground `#737373` on white is approximately 4.74:1

Disabled controls are not communicated by opacity alone; they get explicit disabled foreground/background colors so their labels remain readable.

## Component rule

Before adding a new literal color, ask whether it describes a semantic role already represented by a token. Literal colors are appropriate for content visualization (charts, media, syntax samples) but generally not for window chrome, dialog surfaces, controls, menus, actions or status text.
