# Quick Access Implementation Summary

## Overview

Quick Access is the address-bar overlay that provides one-step navigation to system folders, pinned paths, frequent locations, and recent locations.

This file is a focused implementation note for the Quick Access slice. Repository-wide navigation status and architecture context now live in `NAVIGATION_FEATURE_STATUS.md`, `SHORTCUTS_AND_HANDLERS.md`, and `INPUT_SYSTEM_ARCHITECTURE.md`.

## Current Scope

Quick Access currently provides:

- system folders from the backend special-folder API
- pinned locations synchronized with the address bar pin model
- frequent folders derived from path history
- recent folders derived from path history
- inline filtering with fuzzy ranking across section items
- direct `Alt+1` through `Alt+9` item activation within the active section
- scoped keyboard handling through the input engine overlay context

## Backend Boundary

Quick Access depends on the special-folder backend boundary:

- `special_folders.go`
- `special_folders_windows.go`
- `special_folders_unix.go`

The Wails-facing API returns `SpecialFoldersResponse`, with `SpecialFolder` shared to the frontend through `frontend/src/app/shared/protocol.ts`.

## Frontend Boundary

The frontend implementation is split across:

- `frontend/src/app/components/address-bar-quick-access.component.ts` for the overlay surface
- `frontend/src/app/components/address-bar.component.ts` for host integration and dropdown coordination
- `frontend/src/app/services/path-history.service.ts` for recent and frequent data
- `frontend/src/app/services/input-engine.service.ts` and `frontend/src/app/services/input-context.service.ts` for scoped overlay bindings

## Input Integration

Quick Access now participates in the shared input system under the overlay context ID `overlay.quickAccess`.

Current command surface:

- `Escape` closes the overlay
- `Ctrl+F` focuses the filter field
- `Tab` and `Shift+Tab` switch sections
- `ArrowUp` and `ArrowDown` move the active item
- `Enter` opens the selected item
- `Delete` removes the selected pinned item when applicable
- `Alt+1` through `Alt+9` open the visible item in the corresponding slot of the active section

Filtering notes:

- filter matches are now fuzzy-ranked rather than plain substring matches
- item names and pinned labels are weighted above raw paths so likely destinations float to the top first
- section order stays stable while item order inside each visible section is ranked by relevance

The architectural source of truth for those bindings is `INPUT_SYSTEM_ARCHITECTURE.md`.

## Notes

- This document intentionally avoids point-in-time completion percentages because the broader navigation status is tracked elsewhere.
- If Quick Access behavior changes, update this file only for implementation-specific notes; update the status matrix and input binding docs separately.
