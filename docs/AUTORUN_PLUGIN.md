# Autorun Plugin

`builtin.autorun` contributes the `autorun://` PanelSource.

It is an aggregating provider rather than a second registry/filesystem implementation:

- `autorun://user/registry-run` — HKCU Run
- `autorun://user/registry-run-once` — HKCU RunOnce
- `autorun://user/startup-folder` — current-user Startup folder
- equivalent `machine` locations for all users

Registry entries are disabled losslessly by moving their value into a sibling
`*.DisabledByReassembly` key while preserving REG_SZ vs REG_EXPAND_SZ. Re-enabling
moves the value back. Startup-folder entries are disabled by reversible filename
suffixing.

The plugin contributes Enabled, Source, Scope and Command columns plus actions for
enable/disable, opening the resolved target, and permanent removal.

On non-Windows platforms the backend returns an empty list and mutation methods
report that autorun management is Windows-only.
