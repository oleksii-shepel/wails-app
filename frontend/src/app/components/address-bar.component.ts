import {
  Component, Input, Output, EventEmitter,
  OnInit, OnDestroy, OnChanges, SimpleChanges,
  ViewChild, ElementRef, ChangeDetectorRef,
  HostListener,
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { EMPTY, Subject, debounceTime, distinctUntilChanged, filter, from, merge, switchMap, takeUntil } from 'rxjs';
import { DropdownDirective, DropdownTriggerDirective, DropdownMenuDirective } from '../directives/dropdown.directive';

import { PathHistoryService, PathSuggestion } from '../services/path-history.service';
import { ApiService } from '../services/api.service';
import { DirectoryListing, FileType } from '../shared/protocol';
import { DriveInfo } from '../shared/protocol-enhanced';
import { ArchiveService, ArchiveFrame, isArchive } from '../services/archive.service';
import { parsePathWithArchives, buildSegmentsFromParsedPath } from './address-bar.utils';

import { PathSegment, SiblingEntry, PinnedPath, ContextMenuState, PINNED_KEY, CTX_MENU_CLOSED } from './address-bar.types';
import { isWindowsPath, normalizePath, parentPath, shortenPath, buildSegments } from './address-bar.utils';
import { AddressBarEditComponent } from './address-bar-edit.component';
import { AddressBarPinDropdownComponent } from './address-bar-pin-dropdown.component';
import { AddressBarDriveDropdownComponent } from './address-bar-drive-dropdown.component';
import { AddressBarSiblingDropdownComponent } from './address-bar-sibling-dropdown.component';

// ── Tab-completion session ────────────────────────────────────────────────────
//
// Full state-machine flow:
//
//  [user types "/home/use"]
//      userTyped$ → debounce 1 s (idle-slash pipeline)
//      tryAppendSlash("/home/use")
//        → editValue  = "/home/user/"
//        → tabSession = { candidates:[Documents,Downloads,…], enteredDir:true }
//        → suggestions = children shown in dropdown
//
//  TAB (enteredDir=true)
//      tabForward → applyCandidate(candidates[0])
//        → editValue         = "/home/user/Documents"   (no slash yet)
//        → tabSession.enteredDir = false
//        → tabSelected$.next("/home/user/Documents")    ← restarts 1 s idle
//
//  [user keeps pressing TAB — each TAB resets the 1 s window]
//      TAB idx=1  → "/home/user/Downloads"   tabSelected$ emits
//      TAB idx=2  → "/home/user/Music"        tabSelected$ emits
//      … pause 1 s …
//      tryAppendSlash("/home/user/Music")
//        → editValue  = "/home/user/Music/"
//        → tabSession = { candidates:[Albums,Playlists,…], enteredDir:true }
//
//  TAB (enteredDir=true) → applyCandidate(candidates[0]) "/home/user/Music/Albums"
//
//  Shift+TAB (enteredDir=true)
//      strip slash → "/home/user/Music"
//      reload parent siblings, land on Music
//      tabSession = { candidates:parentSiblings, enteredDir:false }
//      tabSelected$.next("/home/user/Music")
//
//  Shift+TAB (enteredDir=false) → cycle backwards through siblings
//
//  [user types any key]
//      applyingTab=false → onEditChange clears tabSession, emits userTyped$

interface TabSession {
  candidates: PathSuggestion[];
  index: number;
  /**
   * True right after idle-slash appended "/" and loaded children.
   *   TAB       → select candidates[0], set enteredDir=false, restart idle
   *   Shift+TAB → strip the slash, go up to parent siblings, restart idle
   */
  enteredDir: boolean;
}

// A small tagged wrapper so the idle-slash pipeline knows which subject fired.
// This lets us drop distinctUntilChanged on the TAB side (same path can be
// re-selected after going away and coming back) while keeping it on the
// typing side (no point re-fetching suggestions for the same typed string).
interface IdleSlashTick {
  value: string;
  /** true = came from TAB selection; false = came from user keystroke */
  fromTab: boolean;
}

@Component({
  selector: 'app-address-bar',
  standalone: true,
  imports: [
    CommonModule,
    DropdownDirective,
    DropdownTriggerDirective,
    DropdownMenuDirective,
    AddressBarSiblingDropdownComponent,
    AddressBarPinDropdownComponent,
    AddressBarDriveDropdownComponent,
    AddressBarEditComponent,
  ],
  template: `
<div class="ab-host" (keydown)="onHostKeyDown($event)">

  <!-- ── BREADCRUMB ROW ──────────────────────────────────────────────── -->
  <div class="ab-bar" *ngIf="!editMode" [class.ab-bar--compact]="compact">

    <!-- Left cluster: pin star + pin dropdown -->
    <div class="ab-cluster ab-cluster--left">
      <button
        class="ab-btn ab-btn--star"
        [class.ab-btn--pinned]="isPinned"
        (click)="togglePin()"
        [title]="isPinned ? 'Unpin this path' : 'Pin this path'"
        aria-label="Toggle pin"
      >{{ isPinned ? '★' : '☆' }}</button>

      <div class="ab-drop-anchor" appDropdown [(isOpen)]="showPinDrop">
        <button
          appDropdownTrigger
          class="ab-btn ab-btn--pinlist"
          [class.ab-btn--open]="showPinDrop"
          (click)="togglePinDrop()"
          title="Pinned paths"
          aria-label="Open pinned paths"
        >
          <svg width="13" height="13" viewBox="0 0 16 16" fill="currentColor">
            <path d="M9.5 1.5a1 1 0 0 0-1.415 0L4.94 4.645a3.5 3.5 0 0 0-.871 3.547l-2.646 2.647a.5.5 0 0 0 .708.707l2.646-2.646a3.5 3.5 0 0 0 3.547-.872l3.144-3.144A1 1 0 0 0 11.5 3l-2-1.5z"/>
          </svg>
        </button>
        <app-address-bar-pin-dropdown
          appDropdownMenu
          *ngIf="showPinDrop"
          [pins]="pinnedPaths"
          [currentPath]="currentPath"
          (navigate)="navigateTo($event)"
          (remove)="removePin($event.pin, $event.event)"
        />
      </div>
    </div>

    <!-- Breadcrumb trail -->
    <div class="ab-crumbs" (click)="onCrumbsClick($event)">
      <!-- Drive letter stays fixed, outside the scroll -->
      <ng-container *ngIf="isWin && segments.length > 0">
        <span
          class="ab-seg ab-drive-letter"
          [class.ab-seg--last]="segments.length === 1"
          (click)="onDriveLetterClick(segments[0], segments.length === 1, $event)"
          (contextmenu)="onSegRightClick(segments[0], 0, $event)"
          title="Switch drives (single-click) or double-click to edit path"
        >{{ segments[0].label }}</span>

        <div class="ab-drive-chevron-wrap" *ngIf="segments.length > 1">
          <button
            class="ab-chev ab-drive-chevron"
            [class.ab-chev--open]="chevIdx === 0"
            (click)="toggleChev(0, segments[0], $event)"
            [title]="'Show contents of ' + segments[0].fullPath"
            aria-label="Show drive contents"
          >›</button>
        </div>
      </ng-container>

      <!-- Everything else scrolls -->
      <div class="ab-crumbs-inner" #crumbsEl>
        <ng-container *ngFor="let seg of segments; let i = index; let last = last">
          <!-- skip index 0 on Windows (drive letter rendered above) -->
          <ng-container *ngIf="!isWin || i > 0">

            <!-- 
              ARCHIVE FILE (can be both root AND inner)
              - Every archive file gets isArchiveRoot=true
              - If it's inside another archive, also gets isArchiveInner=true
            -->
            <span
              *ngIf="seg.isArchiveRoot"
              class="ab-seg"
              [class.ab-seg--archive-root]="seg.isArchiveRoot"
              [class.ab-seg--archive-inner]="seg.isArchiveInner"
              [class.ab-seg--clickable]="!last"
              [class.ab-seg--last]="last"
              (click)="onArchiveRootSegClick(seg, last, $event)"
              (contextmenu)="onSegRightClick(seg, i, $event)"
              [title]="last ? 'Click to edit path' : seg.fullPath"
            >
              <ng-container *ngIf="getSegmentLabelParts(seg) as labelParts">
                <span class="ab-seg-label">
                  <span class="ab-seg-main">{{ labelParts.main }}</span>
                  <span *ngIf="labelParts.ext" class="ab-seg-ext">{{ labelParts.ext }}</span>
                </span>
              </ng-container>
            </span>

            <!-- REGULAR FOLDER INSIDE ARCHIVE (not an archive file) -->
            <span
              *ngIf="seg.isArchiveInner && !seg.isArchiveRoot"
              class="ab-seg ab-seg--archive-inner"
              [class.ab-seg--last]="last"
              [class.ab-seg--clickable]="!last"
              [title]="last ? 'Click to edit path' : seg.fullPath"
              (click)="onArchiveInnerSegClick(seg, last, $event)"
              (contextmenu)="onSegRightClick(seg, i, $event)"
            >
              <ng-container *ngIf="getSegmentLabelParts(seg) as labelParts">
                <span class="ab-seg-label">
                  <span class="ab-seg-main">{{ labelParts.main }}</span>
                  <span *ngIf="labelParts.ext" class="ab-seg-ext">{{ labelParts.ext }}</span>
                </span>
              </ng-container>
            </span>

            <!-- REGULAR FS SEGMENT (non-archive, non-Windows-root) -->
            <span
              *ngIf="(!seg.isRoot || !isWin) && !seg.isArchiveInner && !seg.isArchiveRoot"
              class="ab-seg"
              [class.ab-seg--root]="seg.isRoot"
              [class.ab-seg--last]="last"
              (click)="onSegClick(seg, $event)"
              (contextmenu)="onSegRightClick(seg, i, $event)"
              [title]="last ? 'Click to edit path' : seg.fullPath"
            >
              <ng-container *ngIf="getSegmentLabelParts(seg) as labelParts">
                <span class="ab-seg-label">
                  <span class="ab-seg-main">{{ labelParts.main }}</span>
                  <span *ngIf="labelParts.ext" class="ab-seg-ext">{{ labelParts.ext }}</span>
                </span>
              </ng-container>
            </span>

            <!-- Chevron after segment (if not last) -->
            <div class="ab-chev-wrap" *ngIf="!last">
              <button
                class="ab-chev"
                [class.ab-chev--archive]="seg.isArchiveRoot || seg.isArchiveInner"
                [class.ab-chev--open]="chevIdx === i"
                (click)="toggleChev(i, seg, $event)"
                [attr.aria-label]="(seg.isArchiveRoot || seg.isArchiveInner) ? 'Browse archive' : 'Browse siblings'"
              >›</button>
            </div>

          </ng-container>
        </ng-container>
      </div>
    </div>

    <!-- Right cluster: Up / Refresh / Copy -->
    <div class="ab-cluster ab-cluster--right">
      <button class="ab-btn" (click)="goUp()" title="Parent directory" aria-label="Go up">
        <svg width="13" height="13" viewBox="0 0 16 16" fill="currentColor">
          <path d="M8 1a.5.5 0 0 1 .5.5v11.793l3.146-3.147a.5.5 0 0 1 .708.708l-4 4a.5.5 0 0 1-.708 0l-4-4a.5.5 0 0 1 .708-.708L7.5 13.293V1.5A.5.5 0 0 1 8 1z" transform="rotate(180,8,8)"/>
        </svg>
      </button>
      <button class="ab-btn" (click)="refresh()" title="Refresh (F5)" aria-label="Refresh">
        <svg width="13" height="13" viewBox="0 0 16 16" fill="currentColor">
          <path d="M8 3a5 5 0 1 0 4.546 2.914.5.5 0 0 1 .908-.417A6 6 0 1 1 8 2v1z"/>
          <path d="M8 4.466V.534a.25.25 0 0 1 .41-.192l2.36 1.966c.12.1.12.284 0 .384L8.41 4.658A.25.25 0 0 1 8 4.466z"/>
        </svg>
      </button>
      <button class="ab-btn" (click)="copyPath()" [class.ab-btn--copied]="justCopied" title="Copy path" aria-label="Copy path">
        <svg *ngIf="!justCopied" width="13" height="13" viewBox="0 0 16 16" fill="currentColor">
          <path d="M4 1.5H3a2 2 0 0 0-2 2V14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V3.5a2 2 0 0 0-2-2h-1v1h1a1 1 0 0 1 1 1V14a1 1 0 0 1-1 1H3a1 1 0 0 1-1-1V3.5a1 1 0 0 1 1-1h1v-1z"/>
          <path d="M9.5 1a.5.5 0 0 1 .5.5v1a.5.5 0 0 1-.5.5h-3a.5.5 0 0 1-.5-.5v-1a.5.5 0 0 1 .5-.5h3zm-3-1A1.5 1.5 0 0 0 5 1.5v1A1.5 1.5 0 0 0 6.5 4h3A1.5 1.5 0 0 0 11 2.5v-1A1.5 1.5 0 0 0 9.5 0h-3z"/>
        </svg>
        <svg *ngIf="justCopied" width="13" height="13" viewBox="0 0 16 16" fill="currentColor">
          <path d="M13.485 1.929L5.5 9.914 2.515 6.929a1 1 0 0 0-1.414 1.414l3.692 3.693a1 1 0 0 0 1.414 0l8.692-8.693a1 1 0 0 0-1.414-1.414z"/>
        </svg>
      </button>
    </div>
  </div>

  <!-- ── EDIT MODE ─────────────────────────────────────────────────────── -->
  <address-bar-edit
    *ngIf="editMode"
    [value]="editValue"
    [currentPath]="currentPath"
    [placeholder]="placeholder"
    [suggestions]="suggestions"
    [acIdx]="acIdx"
    [focusOnInit]="editMode"
    (valueChange)="onEditChange($event)"
    (editKeydown)="onEditKeyDown($event)"
    (blur)="onEditBlur()"
    (cancel)="exitEditMode(false)"
    (acIdxChange)="acIdx = $event"
    (suggestionSelect)="acceptSuggestion($event.suggestion, $event.event)"
  />

  <!-- ── SIBLING DROPDOWN ──────────────────────────────────────────────── -->
  <app-address-bar-sibling-dropdown
    appDropdown
    [(isOpen)]="sibOpen"
    (isOpenChange)="onSibOpenChange($event)"
    *ngIf="chevIdx >= 0"
    [entries]="siblings"
    [loading]="sibLoading"
    [left]="sibDropLeft"
    [top]="sibDropTop"
    [headLabel]="segments[chevIdx].label"
    (entryClick)="onSibClick($event)"
  />

  <!-- ── DRIVE DROPDOWN ────────────────────────────────────────────────── -->
  <app-address-bar-drive-dropdown
    appDropdown
    [(isOpen)]="showDriveMenu"
    *ngIf="showDriveMenu"
    [drives]="drives"
    [loading]="drivesLoading"
    [left]="driveMenuLeft"
    [top]="driveMenuTop"
    [currentPath]="currentPath"
    (driveClick)="switchToDrive($event)"
  />

  <!-- ── CONTEXT MENU ──────────────────────────────────────────────────── -->
  <div
    class="ab-ctx-menu"
    *ngIf="ctxMenu.visible"
    [style.left.px]="ctxMenu.x"
    [style.top.px]="ctxMenu.y"
  >
    <div class="ab-ctx-path">{{ ctxMenu.segment?.fullPath }}</div>
    <hr class="ab-ctx-divider"/>
    <button class="ab-ctx-item" (click)="ctxNavigateTo()">
      <svg width="12" height="12" viewBox="0 0 16 16" fill="currentColor"><path d="M1 3.5A1.5 1.5 0 0 1 2.5 2h2.764c.958 0 1.76.56 2.311 1.184C7.985 3.648 8.48 4 9 4h4.5A1.5 1.5 0 0 1 15 5.5v7a1.5 1.5 0 0 1-1.5 1.5h-11A1.5 1.5 0 0 1 1 12.5v-9z"/></svg>
      Navigate here
    </button>
    <button class="ab-ctx-item" (click)="ctxCopyPath()">
      <svg width="12" height="12" viewBox="0 0 16 16" fill="currentColor"><path d="M4 1.5H3a2 2 0 0 0-2 2V14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V3.5a2 2 0 0 0-2-2h-1v1h1a1 1 0 0 1 1 1V14a1 1 0 0 1-1 1H3a1 1 0 0 1-1-1V3.5a1 1 0 0 1 1-1h1v-1z"/><path d="M9.5 1a.5.5 0 0 1 .5.5v1a.5.5 0 0 1-.5.5h-3a.5.5 0 0 1-.5-.5v-1a.5.5 0 0 1 .5-.5h3zm-3-1A1.5 1.5 0 0 0 5 1.5v1A1.5 1.5 0 0 0 6.5 4h3A1.5 1.5 0 0 0 11 2.5v-1A1.5 1.5 0 0 0 9.5 0h-3z"/></svg>
      Copy path
    </button>
    <button class="ab-ctx-item" (click)="ctxCopyName()">
      <svg width="12" height="12" viewBox="0 0 16 16" fill="currentColor"><path d="M14 4.5V14a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V2a2 2 0 0 1 2-2h5.5L14 4.5zm-3 0A1.5 1.5 0 0 1 9.5 3V1H4a1 1 0 0 0-1 1v12a1 1 0 0 0 1 1h8a1 1 0 0 0 1-1V4.5h-2z"/></svg>
      Copy folder name
    </button>
    <hr class="ab-ctx-divider"/>
    <button class="ab-ctx-item" (click)="ctxOpenOtherPane()">
      <svg width="12" height="12" viewBox="0 0 16 16" fill="currentColor"><path d="M0 3a2 2 0 0 1 2-2h12a2 2 0 0 1 2 2v10a2 2 0 0 1-2 2H2a2 2 0 0 1-2-2V3zm8.5-1v12H14a1 1 0 0 0 1-1V3a1 1 0 0 0-1-1H8.5zM7.5 2H2a1 1 0 0 0-1 1v10a1 1 0 0 0 1 1h5.5V2z"/></svg>
      Open in other pane
    </button>
    <button class="ab-ctx-item" (click)="ctxPinPath()">
      <svg width="12" height="12" viewBox="0 0 16 16" fill="currentColor"><path d="M9.5 1.5a1 1 0 0 0-1.415 0L4.94 4.645a3.5 3.5 0 0 0-.871 3.547l-2.646 2.647a.5.5 0 0 0 .708.707l2.646-2.646a3.5 3.5 0 0 0 3.547-.872l3.144-3.144A1 1 0 0 0 11.5 3l-2-1.5z"/></svg>
      {{ isSegmentPinned(ctxMenu.segment) ? 'Unpin path' : 'Pin path' }}
    </button>
  </div>

</div>
  `,
  styles: [`
    :host {
      display: block;
      position: relative;
      font-family: var(--vsc-font-family);
      font-size: var(--vsc-font-size);
      color: var(--vsc-foreground);
      background: var(--vsc-sidebar-background);
    }

    .ab-host {
      position: relative;
      background: var(--vsc-sidebar-background);
      border-bottom: 1px solid var(--vsc-border-subtle);
    }

    .ab-bar {
      display: flex; align-items: center;
      height: var(--vsc-titlebar-height);
      padding: 0 var(--vsc-padding-sm);
      gap: var(--vsc-padding-xs);
      overflow: visible;
      background: var(--vsc-sidebar-background);
      transition: background var(--vsc-transition-fast);
      user-select: none;
    }

    .ab-bar--compact {
      height: 28px;
      .ab-btn { width: 22px; height: 22px; }
      .ab-seg { height: 20px; line-height: 20px; font-size: var(--vsc-font-size-small); }
    }

    .ab-cluster {
      display: flex; align-items: center;
      gap: var(--vsc-padding-xs); flex-shrink: 0;
    }

    .ab-cluster--left {
      padding-right: var(--vsc-padding-sm);
      border-right: 1px solid var(--vsc-border-subtle);
      margin-right: var(--vsc-padding-sm);
    }

    .ab-cluster--right {
      padding-left: var(--vsc-padding-sm);
      border-left: 1px solid var(--vsc-border-subtle);
      margin-left: var(--vsc-padding-sm);
    }

    .ab-btn {
      display: flex; align-items: center; justify-content: center;
      width: 26px; height: 26px; padding: 0;
      background: transparent; border: none;
      border-radius: var(--vsc-border-radius-sm);
      color: var(--vsc-foreground-dim); cursor: pointer;
      transition: background-color var(--vsc-transition-fast), color var(--vsc-transition-fast);
      flex-shrink: 0;

      &:hover { background: var(--vsc-list-hover-background); color: var(--vsc-foreground-bright); }
      &:active { background: var(--vsc-list-active-background); color: var(--vsc-foreground-bright); }
      &:focus-visible { outline: 1px solid var(--vsc-focusBorder, var(--vsc-accent-blue)); outline-offset: -1px; }

      svg { width: 16px; height: 16px; fill: currentColor; opacity: 0.9; transition: opacity var(--vsc-transition-fast), filter 120ms ease; }
      &:hover svg { opacity: 1; }
      /* Soften icons when button is in an active/open state */
      &.ab-btn--open svg,
      &.ab-btn--pinned svg {
        opacity: 0.78;
        filter: saturate(0.92) brightness(0.94);
      }
    }

    .ab-btn--star { font-size: 16px; line-height: 1; &:hover { color: var(--vsc-accent-yellow); } }
    .ab-btn--pinned { color: var(--vsc-accent-yellow) !important; }
    .ab-btn--open { background: var(--vsc-list-active-background); color: var(--vsc-foreground-bright); svg { opacity: 1; } }
    .ab-btn--copied { color: var(--vsc-success) !important; }

    .ab-drop-anchor { position: relative; }

    .ab-crumbs {
      flex: 1;
      min-width: 0;
      display: flex;
      align-items: center;
      overflow: hidden;
    }

    .ab-crumbs-inner {
      flex: 1 1 0%;
      min-width: 0;
      width: 0;
      display: flex;
      align-items: center;
      height: 100%;
      overflow-x: auto;
      overflow-y: hidden;
      scroll-behavior: auto;
      scrollbar-width: none;
      -ms-overflow-style: none;
      &::-webkit-scrollbar { display: none; }

      > * { flex-shrink: 0; }
    }

    .ab-drive-letter {
      flex-shrink: 0;
    }

    .ab-seg {
      display: inline-flex; align-items: center;
      height: 24px; padding: 0 var(--vsc-padding-sm);
      border-radius: var(--vsc-border-radius-sm);
      color: var(--vsc-foreground-dim); font-size: var(--vsc-font-size);
      line-height: 24px; cursor: pointer;
      transition: background-color var(--vsc-transition-fast), color var(--vsc-transition-fast);
      max-width: 260px; overflow: hidden;

      &:hover { background: var(--vsc-list-hover-background); color: var(--vsc-foreground); }
      
      &.ab-seg--root { color: var(--vsc-foreground); font-weight: 500; }
      
      /* Last segment: clicking enters edit mode rather than navigating */
      &.ab-seg--last {
        color: var(--vsc-foreground-bright); cursor: text; max-width: 320px;
        &:hover { background: color-mix(in srgb, var(--vsc-accent-blue) 12%, transparent); }
      }

      /* ARCHIVE ROOT - any archive file in the path */
      &.ab-seg--archive-root {
        background: color-mix(in srgb, var(--vsc-accent-orange) 18%, transparent);
        border: 1px solid rgba(255, 140, 0, 0.35);
        color: #ffb347;
        gap: 4px;
        
        .ab-seg-archive-icon { 
          opacity: 0.9; 
          flex-shrink: 0;
          font-size: 14px;
        }
        
        &.ab-seg--clickable { cursor: pointer; }
        &.ab-seg--clickable:hover { background: color-mix(in srgb, var(--vsc-accent-orange) 34%, transparent); }
        
        /* NESTED ARCHIVE - archive inside another archive */
        &.ab-seg--archive-inner {
          background: color-mix(in srgb, var(--vsc-accent-orange) 24%, transparent);
          
          &:hover { background: color-mix(in srgb, var(--vsc-accent-orange) 40%, transparent); }
        }
      }

      /* REGULAR FOLDER INSIDE ARCHIVE (not an archive file) */
      &.ab-seg--archive-inner:not(.ab-seg--archive-root) {
        color: color-mix(in srgb, #ffb347 80%, var(--vsc-foreground));
        cursor: default;
        
        &.ab-seg--clickable { cursor: pointer; }
        &.ab-seg--clickable:hover { 
          background: color-mix(in srgb, #ffb347 15%, transparent); 
          color: #ffb347; 
        }
        &:not(.ab-seg--clickable):hover { background: transparent; }
      }
    }

    .ab-drive-letter {
      /* Softer drive letter styling: lower opacity and subtler border */
      background: color-mix(in srgb, #007fd4 12%, transparent);
      border: 1px solid rgba(0,127,212,0.25);
      border-radius: 4px;
      color: #6fb3db; /* slightly dimmer blue */
      font-family: var(--vsc-font-family-mono); font-weight: 600;
      padding: 0 var(--vsc-padding-lg) !important; margin-right: var(--vsc-padding-md) !important;
      cursor: pointer; transition: background 0.12s, border-color 0.12s, color 120ms;
      &:hover { background: color-mix(in srgb, #007fd4 20%, transparent); border-color: rgba(0,127,212,0.38); }
      &.ab-seg--last { background: color-mix(in srgb, #007fd4 10%, transparent); cursor: pointer; }
    }

    .ab-chev-wrap {
      position: relative; flex-shrink: 0; overflow: visible;
      display: inline-flex; align-items: center;
    }

    .ab-chev {
      display: flex; align-items: center; justify-content: center;
      width: 20px; height: 24px; padding: 0;
      background: transparent !important; border: none;
      color: var(--vsc-foreground-dim); font-size: 16px; font-weight: 400;
      cursor: pointer; border-radius: var(--vsc-border-radius-sm);
      transition: color var(--vsc-transition-fast), transform var(--vsc-transition-normal);

      &:hover { color: var(--vsc-foreground); background: var(--vsc-list-hover-background) !important; }
      &:focus-visible { outline: 1px solid var(--vsc-accent-blue); outline-offset: -1px; }
      &.ab-chev--open { transform: rotate(90deg); color: var(--vsc-foreground-dim); background: transparent !important; }

      /* Archive chevron styling */
      &.ab-chev--archive {
        color: rgba(255, 179, 71, 0.6);
        background: transparent !important;
        
        &:hover { 
          background: transparent !important; 
          color: #ffb347; 
        }
        
        &.ab-chev--open { 
          transform: rotate(90deg); 
          color: #ffb347; 
          background: transparent !important; 
        }
      }
    }

    .ab-drive-chevron-wrap { display: inline-flex; align-items: center; margin-right: var(--vsc-padding-xs); }
    .ab-drive-chevron {
      border-radius: 0 var(--vsc-border-radius-sm) var(--vsc-border-radius-sm) 0; margin-left: -2px;
      background: transparent !important;
      &.ab-chev--open { background: transparent !important; color: var(--vsc-foreground-dim); }
    }

    /* Context menu */
    .ab-ctx-menu {
      position: fixed; min-width: 200px;
      background: var(--vsc-panel-background); border: 1px solid var(--vsc-border);
      border-radius: var(--vsc-border-radius); box-shadow: var(--vsc-widget-shadow);
      z-index: calc(var(--vsc-z-modal) + 100);
      padding: var(--vsc-padding-xs) 0;
      animation: ab-ctx-appear var(--vsc-transition-fast) cubic-bezier(0.2, 0, 0.2, 1);
    }

    @keyframes ab-ctx-appear {
      from { opacity: 0; transform: translateY(-4px); }
      to   { opacity: 1; transform: translateY(0); }
    }

    .ab-ctx-path {
      padding: var(--vsc-padding-sm) var(--vsc-padding-lg);
      font-size: 11px; font-family: var(--vsc-font-family-mono);
      color: var(--vsc-foreground-dim); white-space: nowrap; overflow: hidden;
      text-overflow: ellipsis; max-width: 240px;
      border-bottom: 1px solid var(--vsc-border-subtle);
      margin-bottom: var(--vsc-padding-xs);
    }

    .ab-ctx-divider { margin: var(--vsc-padding-xs) 0; border: none; border-top: 1px solid var(--vsc-border-subtle); }

    .ab-ctx-item {
      display: flex; align-items: center; gap: var(--vsc-padding-sm);
      width: 100%; padding: var(--vsc-padding-sm) var(--vsc-padding-lg);
      background: transparent; border: none; color: var(--vsc-foreground);
      font-size: var(--vsc-font-size); font-family: var(--vsc-font-family);
      cursor: pointer; text-align: left;
      transition: background-color var(--vsc-transition-fast);
      svg { width: 14px; height: 14px; fill: currentColor; opacity: 0.8; flex-shrink: 0; }
      &:hover { background: var(--vsc-list-hover-background); svg { opacity: 1; } }
      &:active { background: var(--vsc-list-active-background); color: var(--vsc-foreground-bright); }
    }

    .ab-seg-label {
      display: inline-flex;
      align-items: center;
      min-width: 0;
      width: 100%;
    }

    .ab-seg-main {
      flex: 1 1 auto;
      min-width: 0;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }

    .ab-seg-ext {
      flex: 0 0 auto;
    }

    /* Compact mode */
    :host-context(.compact-mode) {
      .ab-bar { height: 28px; }
      .ab-btn { width: 22px; height: 22px; svg { width: 14px; height: 14px; } }
      .ab-chev { width: 18px; height: 22px; }
      .ab-seg { height: 22px; line-height: 22px; font-size: var(--vsc-font-size-small); padding: 0 var(--vsc-padding-sm); }
    }

    /* Light theme adjustments */
    :host-context(.vscode-light) {
      .ab-drive-letter { background: color-mix(in srgb, #0060c0 10%, transparent); border-color: rgba(0,96,192,0.28); color: #0a6fb0; }
      
      .ab-seg--archive-root {
        background: color-mix(in srgb, #ff8c00 12%, transparent);
        border-color: rgba(255, 140, 0, 0.3);
        color: #b85e00;
        
        &.ab-seg--archive-inner {
          border-color: rgba(255, 140, 0, 0.3);
          background: color-mix(in srgb, #ff8c00 18%, transparent);
        }
      }
      
      .ab-seg--archive-inner:not(.ab-seg--archive-root) {
        color: #b85e00;
      }
      
      .ab-chev--archive {
        color: rgba(184, 94, 0, 0.5);
        &:hover { color: #b85e00; }
        &.ab-chev--open { color: #b85e00; }
      }
    }
  `],
})
export class AddressBarComponent implements OnInit, OnDestroy, OnChanges {
  @ViewChild('crumbsEl') crumbsEl?: ElementRef<HTMLDivElement>;

  @Input() initialPath = '/';
  @Input() placeholder = 'Enter path…';
  @Input() paneId = 'left';
  @Input() tabId = '';

  @Output() pathChange          = new EventEmitter<string>();
  @Output() navigateUpClicked   = new EventEmitter<void>();
  @Output() refreshClicked      = new EventEmitter<void>();
  @Output() openInOtherPane     = new EventEmitter<string>();
  @Output() archiveSegmentClick = new EventEmitter<string>();

  // ── Path state ───────────────────────────────────────────────────────────
  currentPath = '/';
  segments: PathSegment[] = [];
  get isWin(): boolean { return isWindowsPath(this.currentPath); }

  // ── Drive switcher ───────────────────────────────────────────────────────
  drives: DriveInfo[] = [];
  showDriveMenu = false;
  driveMenuLeft = 0;
  driveMenuTop  = 0;
  drivesLoading = false;

  // ── Chevron / sibling dropdown ───────────────────────────────────────────
  chevIdx     = -1;
  sibOpen     = false;
  siblings: SiblingEntry[] = [];
  sibLoading  = false;
  sibDropLeft = 0;
  sibDropTop  = 0;

  // ── Pinned paths ─────────────────────────────────────────────────────────
  pinnedPaths: PinnedPath[] = [];
  showPinDrop = false;
  get isPinned(): boolean { return this.pinnedPaths.some(p => p.path === this.currentPath); }

  // ── Edit mode ────────────────────────────────────────────────────────────
  editMode  = false;
  editValue = '';

  // ── Suggestions (history-based dropdown) ─────────────────────────────────
  suggestions: PathSuggestion[] = [];
  acIdx = -1;

  // ── Context menu ─────────────────────────────────────────────────────────
  ctxMenu: ContextMenuState = { ...CTX_MENU_CLOSED };

  // ── Misc ─────────────────────────────────────────────────────────────────
  justCopied = false;
  compact    = false;

  // ── RxJS ─────────────────────────────────────────────────────────────────

  private readonly destroy$ = new Subject<void>();

  /**
   * Emits only when the user actually types a character.
   * Suppressed when TAB writes to editValue (applyingTab guard).
   * Feeds: history suggestions + idle-slash pipeline.
   */
  private readonly userTyped$ = new Subject<string>();

  /**
   * Emits the selected path each time TAB lands on a candidate.
   * Feeds: idle-slash pipeline only — restarts the 1 s debounce so the
   * slash fires 1 s after the user stops pressing TAB.
   * No distinctUntilChanged — same path can be re-visited legitimately.
   */
  private readonly tabSelected$ = new Subject<string>();

  // ── Tab completion ────────────────────────────────────────────────────────

  private tabSession: TabSession | null = null;

  /**
   * True while TAB logic is writing to editValue.
   * Makes onEditChange skip clearing tabSession / emitting userTyped$.
   */
  private applyingTab = false;

  // ── Timers ───────────────────────────────────────────────────────────────
  private blurTimer:   ReturnType<typeof setTimeout> | undefined;
  private copiedTimer: ReturnType<typeof setTimeout> | undefined;

  // ── Scroll ───────────────────────────────────────────────────────────────
  private scrollAnimFrame: number | null = null;
  private wheelListener?: (e: WheelEvent) => void;

  constructor(
    private pathHistoryService: PathHistoryService,
    private apiService: ApiService,
    private archiveService: ArchiveService,
    private elRef: ElementRef,
    private cdr: ChangeDetectorRef,
  ) {}

  // ── Lifecycle ─────────────────────────────────────────────────────────────

  ngOnInit(): void {
    this.currentPath = normalizePath(this.initialPath);
    this.rebuildSegments();
    this.loadPins();
    if (this.isWin) this.loadDrives();

    // ── Pipeline 1: history suggestions ───────────────────────────────────
    // 240 ms debounce, user-typed only, switchMap cancels stale requests.
    this.userTyped$.pipe(
      debounceTime(240),
      distinctUntilChanged(),
      switchMap(v => v?.trim()
        ? from(this.pathHistoryService.getSuggestions(v, this.currentPath))
        : EMPTY,
      ),
      takeUntil(this.destroy$),
    ).subscribe(suggestions => {
      this.suggestions = suggestions;
      this.acIdx = -1;
      this.cdr.detectChanges();
    });

    // ── Pipeline 2: idle-slash ─────────────────────────────────────────────
    //
    // Fires 1 s after whichever happened last: user typed OR TAB selected.
    // switchMap ensures only the latest fires; prior async calls are cancelled.
    //
    // userTyped$ uses distinctUntilChanged — no point re-running for the same
    // typed string.  tabSelected$ does NOT — same path can be legitimately
    // re-visited after cycling away and back.
    //
    // Guards:
    //   • editMode must still be active
    //   • value must not already end with "/" or be a root
    //   • tabSession must not already be in enteredDir=true (slash was already
    //     appended; we are just waiting for TAB)
    merge(
      this.userTyped$.pipe(distinctUntilChanged()),
      this.tabSelected$,
    ).pipe(
      debounceTime(1000),
      filter(() => this.editMode),
      filter(() => !this.tabSession?.enteredDir),
      filter(v => {
        const n = normalizePath(v);
        return !!v && !n.endsWith('/') && n !== '/' && !/^[A-Za-z]:\/$/.test(n);
      }),
      switchMap(v => from(this.tryAppendSlash(v))),
      takeUntil(this.destroy$),
    ).subscribe();

    document.addEventListener('mousedown', this.onEditModeOutsideClick, { capture: true });
  }

  ngAfterViewInit(): void {
    this.setupWheelScroll();
  }

  ngAfterViewChecked(): void {}

  ngOnChanges(changes: SimpleChanges): void {
    if (changes['tabId'] && !changes['tabId'].firstChange) {
      this.closeAll();
      if (this.editMode) {
        clearTimeout(this.blurTimer);
        this.editMode    = false;
        this.editValue   = '';
        this.suggestions = [];
        this.acIdx       = -1;
        this.tabSession  = null;
      }
      const rawNext = changes['initialPath']?.currentValue ?? this.initialPath;
      this.currentPath = normalizePath(rawNext);
      this.rebuildSegments();
      this.cdr.detectChanges();
      return;
    }

    if (changes['initialPath'] && !changes['initialPath'].firstChange) {
      const next = normalizePath(changes['initialPath'].currentValue);
      if (next !== this.currentPath) {
        this.currentPath = next;
        this.rebuildSegments();
        if (this.editMode) this.editValue = next;
      }
    }
  }

  ngOnDestroy(): void {
    clearTimeout(this.blurTimer);
    clearTimeout(this.copiedTimer);
    if (this.scrollAnimFrame !== null) cancelAnimationFrame(this.scrollAnimFrame);
    this.destroy$.next();
    this.destroy$.complete();
    document.removeEventListener('mousedown', this.onEditModeOutsideClick, { capture: true } as any);
    const el = this.crumbsEl?.nativeElement;
    if (el && this.wheelListener) el.removeEventListener('wheel', this.wheelListener);
  }

  // ── Segments ──────────────────────────────────────────────────────────────

  private rebuildSegments(): void {
    const { fsDirPath, archiveInnerParts } = parsePathWithArchives(this.currentPath);

    this.segments = archiveInnerParts.length > 0
      ? buildSegmentsFromParsedPath(fsDirPath, archiveInnerParts)
      : buildSegments(this.currentPath);

    let insideArchive = false;
    for (let i = 0; i < this.segments.length; i++) {
      const seg = this.segments[i];
      const isRealArchive = isArchive(seg.label);
      this.segments[i] = { ...seg, isArchiveRoot: isRealArchive, isArchiveInner: insideArchive };
      if (isRealArchive) insideArchive = true;
    }
    // Animate scroll to end after segments update (next frame so DOM has rendered)
    requestAnimationFrame(() => this.animateScrollToEnd());
  }

  // ── Segment click handlers ────────────────────────────────────────────────

  onSegClick(seg: PathSegment, e: MouseEvent): void {
    e.stopPropagation();
    this.closeAll();
    if (seg.isArchiveRoot || seg.isArchiveInner) {
      seg.fullPath !== this.currentPath
        ? this.archiveSegmentClick.emit(seg.fullPath)
        : this.enterEditMode();
      return;
    }
    seg.fullPath !== this.currentPath ? this.navigateTo(seg.fullPath) : this.enterEditMode();
  }

  getSegmentLabelParts(seg: PathSegment): { main: string; ext?: string } {
    if (!seg) return { main: '' };
    const cleaned  = (seg.fullPath || seg.label || '').replace(/\/+$/, '');
    const lastPart = cleaned.split('/').pop() || seg.label || '';
    const idx = lastPart.lastIndexOf('.');
    if (idx > 0 && idx < lastPart.length - 1) {
      return { main: lastPart.slice(0, idx), ext: lastPart.slice(idx) };
    }
    return { main: lastPart };
  }

  onArchiveInnerSegClick(seg: PathSegment, isLast: boolean, e: MouseEvent): void {
    if (isLast) { this.enterEditMode(); return; }
    e.stopPropagation();
    this.closeAll();
    this.archiveSegmentClick.emit(seg.fullPath);
  }

  onArchiveRootSegClick(seg: PathSegment, isLast: boolean, e: MouseEvent): void {
    if (isLast) { this.enterEditMode(); return; }
    e.stopPropagation();
    this.closeAll();
    this.archiveSegmentClick.emit(seg.fullPath);
  }

  onDriveLetterClick(seg: PathSegment, isLast: boolean, e: MouseEvent): void {
    // Drive letter always opens the drive switcher — even when at the root
    // of a drive (isLast=true). Unlike regular segments, there is no useful
    // "edit the last crumb" behaviour for a drive letter.
    e.stopPropagation();
    if (this.showDriveMenu) { this.showDriveMenu = false; return; }
    this.closeAll();
    const rect = (e.target as HTMLElement).getBoundingClientRect();
    this.driveMenuLeft = rect.left;
    this.driveMenuTop  = rect.bottom + 4;
    this.showDriveMenu = true;
    if (this.drives.length === 0 && !this.drivesLoading) this.loadDrives();
  }

  onCrumbsClick(e: MouseEvent): void {
    const t = e.target as HTMLElement;
    if (t.classList.contains('ab-crumbs') || t.classList.contains('ab-crumbs-inner')) {
      e.stopPropagation();
      this.enterEditMode();
    }
  }

  private setupWheelScroll(): void {
    const el = this.crumbsEl?.nativeElement;
    if (!el) return;
    this.wheelListener = (e: WheelEvent) => {
      e.preventDefault();
      const delta = e.deltaY || e.deltaX;
      this.animateScrollBy(el, delta * 1.4);
    };
    el.addEventListener('wheel', this.wheelListener, { passive: false });
  }

  /**
   * Smoothly scroll the crumbs bar to its rightmost position.
   * Uses a short ease-out animation so new path segments slide into view
   * rather than snapping. Cancels any in-flight animation first.
   */
  private animateScrollToEnd(): void {
    const el = this.crumbsEl?.nativeElement;
    if (!el) return;
    const target = el.scrollWidth - el.clientWidth;
    if (target <= 0) return;
    this.animateScrollTo(el, target);
  }

  /**
   * Animate scrollLeft from current position to `target` in ~120 ms (ease-out).
   */
  private animateScrollTo(el: HTMLElement, target: number): void {
    if (this.scrollAnimFrame !== null) {
      cancelAnimationFrame(this.scrollAnimFrame);
      this.scrollAnimFrame = null;
    }

    const start     = el.scrollLeft;
    const distance  = target - start;
    if (Math.abs(distance) < 1) return;

    const duration  = Math.min(220, Math.max(160, Math.abs(distance) * 0.3)); // 60-220 ms
    const startTime = performance.now();

    const tick = (now: number) => {
      const elapsed  = now - startTime;
      const progress = Math.min(elapsed / duration, 1);
      // Ease-out cubic: fast start, gentle finish
      const eased    = 1 - Math.pow(1 - progress, 3);
      el.scrollLeft  = start + distance * eased;

      if (progress < 1) {
        this.scrollAnimFrame = requestAnimationFrame(tick);
      } else {
        this.scrollAnimFrame = null;
      }
    };

    this.scrollAnimFrame = requestAnimationFrame(tick);
  }

  /**
   * Animate a scroll-by delta from the current position — used by wheel events
   * for a smooth, momentum-like feel without OS-level momentum scrolling.
   */
  private animateScrollBy(el: HTMLElement, delta: number): void {
    const target = Math.max(0, Math.min(el.scrollWidth - el.clientWidth, el.scrollLeft + delta));
    this.animateScrollTo(el, target);
  }

  // ── Drive handling ────────────────────────────────────────────────────────

  switchToDrive(drive: DriveInfo): void {
    this.showDriveMenu = false;
    const raw = (drive.path || drive.name || '').toString();
    let target = raw;
    if (/^[A-Za-z]:/.test(raw)) {
      target = raw.slice(0, 2) + '/';
    } else {
      const n = raw.replace(/\\+/g, '/');
      if (n.startsWith('//')) {
        const parts = n.split('/').filter(Boolean);
        target = parts.length >= 2 ? '//' + parts[0] + '/' + parts[1] : n;
      }
    }
    if (target && target !== this.currentPath) this.navigateTo(target);
  }

  async loadDrives(): Promise<void> {
    if (!this.isWin) return;
    this.drivesLoading = true;
    try { this.drives = await this.apiService.listDrives() || []; }
    catch { this.drives = []; }
    finally { this.drivesLoading = false; this.cdr.detectChanges(); }
  }

  // ── Chevron / siblings ────────────────────────────────────────────────────

  async toggleChev(idx: number, seg: PathSegment, e: MouseEvent): Promise<void> {
    e.stopPropagation();
    this.showPinDrop   = false;
    this.showDriveMenu = false;

    if (this.chevIdx === idx) { this.chevIdx = -1; this.sibOpen = false; this.siblings = []; return; }

    const rect = (e.target as HTMLElement).getBoundingClientRect();
    this.sibDropLeft = Math.round(rect.left);
    this.sibDropTop  = Math.round(rect.bottom + 4);
    this.chevIdx    = idx;
    this.siblings   = [];
    this.sibLoading = true;

    const activeChildPath = idx + 1 < this.segments.length ? this.segments[idx + 1].fullPath : null;

    try {
      if (seg.isArchiveRoot || seg.isArchiveInner) {
        await this.loadArchiveSiblings(seg, activeChildPath);
        return;
      }
      const listing: DirectoryListing = await this.apiService.listDirectory(seg.fullPath, false);
      this.siblings = listing.entries
        .filter(e => e.type === FileType.DIRECTORY)
        .map(e => ({ name: e.name, path: e.path, isCurrent: e.path === activeChildPath }))
        .sort((a, b) => a.name.localeCompare(b.name));
    } catch {
      this.siblings = [];
    } finally {
      this.sibLoading = false;
      this.cdr.detectChanges();
      this.sibOpen = true;
    }
  }

  private async loadArchiveSiblings(seg: PathSegment, activeChildPath: string | null): Promise<void> {
    const { fsDirPath, archiveInnerParts } = parsePathWithArchives(this.currentPath);
    if (!archiveInnerParts.length) { this.sibLoading = false; return; }

    const syntheticStack: ArchiveFrame[] = [{
      fsPath: fsDirPath, nestedChain: [],
      innerPath: archiveInnerParts.join('/'),
      listing: { archivePath: fsDirPath, innerPath: archiveInnerParts.join('/'), entries: [] },
    }];

    const target = this.archiveService.resolveBrowseTarget(
      { name: seg.label, fullPath: seg.fullPath }, syntheticStack,
    );
    if (!target) { this.sibLoading = false; return; }

    try {
      const listing = await this.archiveService.navigateInto(
        { ...target.frame, nestedChain: target.browseChain }, target.browseInnerPath,
      );
      const activeChildVirtual = activeChildPath?.replace(/\\/g, '/') ?? null;
      const fsPath = fsDirPath.replace(/\\/g, '/');

      this.siblings = listing.listing.entries
        .filter(e => e.type === FileType.DIRECTORY || e.type === FileType.ARCHIVE || isArchive(e.name))
        .map(e => {
          const sibInnerPath = target.browseInnerPath ? `${target.browseInnerPath}/${e.name}` : e.name;
          const chainPart    = target.browseChain.length ? `/${target.browseChain.join('/')}` : '';
          const virtualPath  = `${fsPath}${chainPart}/${sibInnerPath}`.replace(/\/+/g, '/');
          return {
            name: e.name, path: virtualPath,
            isArchive: e.type === FileType.ARCHIVE || isArchive(e.name),
            isCurrent: activeChildVirtual
              ? activeChildVirtual === virtualPath ||
                activeChildVirtual.startsWith(virtualPath + '/') ||
                activeChildVirtual.endsWith('/' + e.name)
              : false,
          } as SiblingEntry;
        })
        .sort((a, b) => a.name.localeCompare(b.name));
    } catch {
      this.siblings = [];
    } finally {
      this.sibLoading = false;
      this.cdr.detectChanges();
    }
  }

  onSibClick(s: SiblingEntry): void {
    this.chevIdx  = -1;
    this.siblings = [];
    const { archiveInnerParts } = parsePathWithArchives(this.currentPath);
    archiveInnerParts.length > 0
      ? this.archiveSegmentClick.emit(s.path)
      : this.navigateTo(s.path);
  }

  onSibOpenChange(open: boolean): void {
    this.sibOpen = open;
    if (!open) { this.chevIdx = -1; this.siblings = []; }
  }

  // ── Pins ──────────────────────────────────────────────────────────────────

  private loadPins(): void {
    try { this.pinnedPaths = JSON.parse(localStorage.getItem(PINNED_KEY) || '[]'); }
    catch { this.pinnedPaths = []; }
  }

  private savePins(): void {
    try { localStorage.setItem(PINNED_KEY, JSON.stringify(this.pinnedPaths)); } catch {}
  }

  togglePin(): void {
    if (this.isPinned) {
      this.pinnedPaths = this.pinnedPaths.filter(p => p.path !== this.currentPath);
    } else {
      const parts = this.currentPath.split('/').filter(Boolean);
      const label = parts.length
        ? parts[parts.length - 1]
        : (isWindowsPath(this.currentPath) ? this.currentPath.slice(0, 2) : '/');
      this.pinnedPaths.push({ path: this.currentPath, label, icon: '📁', addedAt: Date.now() });
    }
    this.savePins();
  }

  togglePinDrop(): void {
    this.showPinDrop = !this.showPinDrop;
    if (this.showPinDrop) { this.chevIdx = -1; this.showDriveMenu = false; }
  }

  removePin(p: PinnedPath, e: MouseEvent): void {
    e.stopPropagation();
    this.pinnedPaths = this.pinnedPaths.filter(x => x.path !== p.path);
    this.savePins();
  }

  isSegmentPinned(seg: PathSegment | null): boolean {
    return !!seg && this.pinnedPaths.some(p => p.path === seg.fullPath);
  }

  // ── Edit mode ─────────────────────────────────────────────────────────────

  enterEditMode(): void {
    this.closeAll();
    this.editMode    = true;
    this.editValue   = this.currentPath;
    this.suggestions = [];
    this.acIdx       = -1;
    this.tabSession  = null;
    this.cdr.detectChanges();
  }

  exitEditMode(commit: boolean): void {
    clearTimeout(this.blurTimer);
    const val = normalizePath(this.editValue);
    this.editMode    = false;
    this.editValue   = '';
    this.suggestions = [];
    this.acIdx       = -1;
    this.tabSession  = null;
    if (commit && val && val !== this.currentPath) this.navigateTo(val);
  }

  onEditBlur(): void {
    this.blurTimer = setTimeout(() => this.exitEditMode(false), 170);
  }

  /**
   * Called on every input keystroke.
   * Real user input (applyingTab=false): clear tabSession, feed userTyped$.
   * TAB writing (applyingTab=true): no-op — session managed by TAB methods.
   */
  onEditChange(v: string): void {
    this.editValue = v;
    if (!this.applyingTab) {
      this.tabSession = null;
      this.userTyped$.next(v);
    }
  }

  async onEditKeyDown(e: KeyboardEvent): Promise<void> {
    switch (e.key) {
      case 'Enter':
        e.preventDefault();
        if (this.acIdx >= 0 && this.suggestions[this.acIdx]) {
          this.acceptSuggestion(this.suggestions[this.acIdx]);
        } else {
          this.exitEditMode(true);
        }
        break;
      case 'Escape':
        e.preventDefault();
        this.exitEditMode(false);
        break;
      case 'ArrowDown':
        e.preventDefault();
        this.acIdx = Math.min(this.acIdx + 1, this.suggestions.length - 1);
        break;
      case 'ArrowUp':
        e.preventDefault();
        this.acIdx = Math.max(this.acIdx - 1, -1);
        break;
      case 'Tab':
        e.preventDefault();
        await this.cycleTab(e.shiftKey ? -1 : 1);
        break;
    }
  }

  // ── Tab completion ────────────────────────────────────────────────────────

  private async cycleTab(direction: 1 | -1): Promise<void> {
    // Bootstrap a session only when none exists.
    // tryAppendSlash() sets up enteredDir=true + children — preserving that
    // session here is critical; re-fetching would discard the enteredDir state.
    if (!this.tabSession || this.tabSession.candidates.length === 0) {
      // No idle-slash yet — user pressed TAB on a raw typed value.
      // Treat the current editValue as a prefix: list the parent dir,
      // filter by the last segment, give the user siblings to cycle through.
      const candidates = await this.fetchChildren(this.editValue);
      this.tabSession  = { candidates, index: -1, enteredDir: false };
    }

    if (this.tabSession.candidates.length === 0) return;

    // Use editValue directly (preserving trailing slash) for index lookup.
    const currentIndex = this.findSessionIndex(this.editValue);

    direction > 0
      ? this.tabForward(currentIndex)
      : await this.tabBackward(currentIndex);
  }

  /**
   * TAB (forward).
   *
   * enteredDir=true  → idle-slash fired, children are loaded.
   *                    Pick candidates[0], set enteredDir=false.
   *                    Emit tabSelected$ to restart 1 s idle.
   *
   * enteredDir=false → cycle forward through siblings.
   *                    Emit tabSelected$ to restart 1 s idle.
   */
  private tabForward(currentIndex: number): void {
    const session   = this.tabSession!;
    const nextIndex = session.enteredDir
      ? 0
      : (currentIndex + 1) % session.candidates.length;

    this.applyCandidate(session.candidates[nextIndex], session.candidates, false);
  }

  /**
   * Shift+TAB (backward).
   *
   * enteredDir=true  → undo the slash: strip it, reload parent-level siblings,
   *                    land on the current dir in that list, restart idle.
   *
   * enteredDir=false → cycle backward through siblings, restart idle.
   */
  private async tabBackward(currentIndex: number): Promise<void> {
    const session = this.tabSession!;

    if (session.enteredDir) {
      // editValue is e.g. "/home/user/Music/" — strip trailing slash
      const withoutSlash = this.editValue.replace(/\/+$/, '');
      const lastSlash    = withoutSlash.lastIndexOf('/');
      const parentDir    = lastSlash >= 0 ? withoutSlash.slice(0, lastSlash + 1) : '/';

      const siblings   = await this.fetchChildren(parentDir);
      // Find this dir in the parent list so we can highlight it
      const landingIdx = siblings.findIndex(
        c => normalizePath(c.path) === normalizePath(withoutSlash),
      );
      const idx = landingIdx >= 0 ? landingIdx : 0;

      this.writeTabValue(withoutSlash);
      this.tabSession  = { candidates: siblings, index: idx, enteredDir: false };
      this.suggestions = siblings;
      this.acIdx       = idx;
      this.cdr.detectChanges();

      this.tabSelected$.next(withoutSlash);
      return;
    }

    // currentIndex of -1 (nothing highlighted) or 0 (first) both wrap to last
    const prevIndex = currentIndex <= 0
      ? session.candidates.length - 1
      : currentIndex - 1;

    this.applyCandidate(session.candidates[prevIndex], session.candidates, false);
  }

  /**
   * Apply a candidate: write its path to editValue, update session,
   * emit tabSelected$ to restart the 1 s idle-slash timer.
   */
  private applyCandidate(
    candidate: PathSuggestion,
    sessionCandidates: PathSuggestion[],
    enteredDir: boolean,
  ): void {
    if (!candidate) return;
    const index = sessionCandidates.indexOf(candidate);

    this.writeTabValue(candidate.path);
    this.tabSession  = { candidates: sessionCandidates, index, enteredDir };
    this.suggestions = sessionCandidates;
    this.acIdx       = index;
    this.cdr.detectChanges();

    // Restart the idle-slash timer from this selection.
    // tabSelected$ has no distinctUntilChanged — same path can be re-visited.
    this.tabSelected$.next(candidate.path);
  }

  /**
   * Write a value to editValue without emitting userTyped$ or clearing tabSession.
   * The applyingTab flag makes onEditChange a no-op for this write.
   */
  private writeTabValue(value: string): void {
    this.applyingTab = true;
    this.editValue   = value;
    this.applyingTab = false;
  }

  /**
   * Find the index of the current editValue in the active session candidates.
   * Compares without trailing slashes so "/home/user/" matches "/home/user".
   */
  private findSessionIndex(editValue: string): number {
    const clean = normalizePath(editValue);
    return (this.tabSession?.candidates ?? []).findIndex(
      c => normalizePath(c.path) === clean,
    );
  }

  // ── Idle-slash ────────────────────────────────────────────────────────────

  /**
   * Called by the idle-slash pipeline 1 s after the last userTyped$ or
   * tabSelected$ emission.
   *
   * Checks whether `value` is a real directory by listing it.
   * If so: appends "/", loads children, sets up a fresh session with
   * enteredDir=true so the next TAB picks the first child.
   *
   * Always fetches fresh — never reuses tabSession, which holds siblings
   * of the current level, not children of the selected candidate.
   */
  private async tryAppendSlash(value: string): Promise<void> {
    if (!this.editMode) return;

    // List `value` as a directory — if it's not one, the API will throw
    // and we return [].
    const children = await this.fetchChildren(value + '/');
    if (children.length === 0) return;

    // It's a real dir with children — append slash and present children.
    const withSlash = value.endsWith('/') ? value : value + '/';

    this.writeTabValue(withSlash);
    this.tabSession  = { candidates: children, index: -1, enteredDir: true };
    this.suggestions = children;
    this.acIdx       = -1;
    this.cdr.detectChanges();
  }

  // ── Directory fetch ───────────────────────────────────────────────────────

  /**
   * List immediate child directories matching a query.
   *
   * Splits the query into a parent directory and a filename prefix:
   *   "/home/use"   → list "/home/",       filter names starting with "use"
   *   "/home/user/" → list "/home/user/",  no prefix filter (show all children)
   *   "C:"          → list "C:/",          no prefix
   */
  private async fetchChildren(query: string): Promise<PathSuggestion[]> {
    const { dirPath, prefix } = this.splitQueryContext(query);
    if (!dirPath) return [];

    try {
      const listing     = await this.apiService.listDirectory(dirPath, false);
      const prefixLower = prefix.toLowerCase();

      return listing.entries
        .filter(e =>
          e.type === FileType.DIRECTORY &&
          e.name.toLowerCase().startsWith(prefixLower),
        )
        .map(e => ({
          path:        this.joinPath(dirPath, e.name),
          displayName: e.name,
          matchType:   'directory' as const,
          score:       100,
          exists:      true,
        }))
        .sort((a, b) => a.path.localeCompare(b.path));
    } catch {
      return [];
    }
  }

  /**
   * Split a query string into the directory to list and the prefix to match.
   *
   *   "/home/use"  → { dirPath: "/home/",    prefix: "use" }
   *   "/home/"     → { dirPath: "/home/",    prefix: ""    }
   *   "/home/user" → { dirPath: "/home/",    prefix: "user"}  ← prefix filter
   *   "C:"         → { dirPath: "C:/",       prefix: ""    }
   */
  private splitQueryContext(query: string): { dirPath: string; prefix: string } {
    const q = (query || '').replace(/\\/g, '/');
    if (!q) return { dirPath: '/', prefix: '' };
    if (q.endsWith('/')) return { dirPath: q, prefix: '' };

    const lastSlash = q.lastIndexOf('/');
    if (lastSlash >= 0) {
      return { dirPath: q.slice(0, lastSlash + 1), prefix: q.slice(lastSlash + 1) };
    }
    if (/^[A-Za-z]:$/.test(q)) return { dirPath: `${q}/`, prefix: '' };
    return { dirPath: '/', prefix: q };
  }

  private joinPath(dirPath: string, name: string): string {
    const clean  = dirPath.replace(/\/+$/, '');
    const joined = `${clean}/${name}`.replace(/\\/g, '/');
    if (joined.startsWith('//')) return `//${joined.slice(2).replace(/\/{2,}/g, '/')}`;
    return joined.replace(/\/{2,}/g, '/');
  }

  // ── History suggestions ───────────────────────────────────────────────────

  acceptSuggestion(s: PathSuggestion, e?: MouseEvent): void {
    e?.preventDefault();
    clearTimeout(this.blurTimer);
    this.editMode    = false;
    this.editValue   = '';
    this.suggestions = [];
    this.tabSession  = null;
    this.navigateTo(normalizePath(s.path));
  }

  // ── Context menu ──────────────────────────────────────────────────────────

  onSegRightClick(seg: PathSegment, idx: number, e: MouseEvent): void {
    e.preventDefault(); e.stopPropagation();
    this.closeAll();
    this.ctxMenu = { visible: true, x: e.clientX, y: e.clientY, segmentIndex: idx, segment: seg };
  }

  ctxNavigateTo(): void   { if (this.ctxMenu.segment) this.navigateTo(this.ctxMenu.segment.fullPath); this.closeCtx(); }
  async ctxCopyPath(): Promise<void> { if (this.ctxMenu.segment) await navigator.clipboard.writeText(this.ctxMenu.segment.fullPath).catch(() => {}); this.closeCtx(); }
  async ctxCopyName(): Promise<void> { if (this.ctxMenu.segment) await navigator.clipboard.writeText(this.ctxMenu.segment.label).catch(() => {}); this.closeCtx(); }
  ctxOpenOtherPane(): void { if (this.ctxMenu.segment) this.openInOtherPane.emit(this.ctxMenu.segment.fullPath); this.closeCtx(); }

  ctxPinPath(): void {
    const seg = this.ctxMenu.segment;
    if (!seg) { this.closeCtx(); return; }
    const already = this.pinnedPaths.some(p => p.path === seg.fullPath);
    if (already) {
      this.pinnedPaths = this.pinnedPaths.filter(p => p.path !== seg.fullPath);
    } else {
      this.pinnedPaths.push({ path: seg.fullPath, label: seg.label, icon: '📁', addedAt: Date.now() });
    }
    this.savePins();
    this.closeCtx();
  }

  private closeCtx(): void { this.ctxMenu = { ...CTX_MENU_CLOSED }; }

  // ── Navigation ────────────────────────────────────────────────────────────

  goUp(): void {
    this.navigateUpClicked.emit();
    const { archiveInnerParts } = parsePathWithArchives(this.currentPath);
    if (archiveInnerParts.length === 0) {
      const p = parentPath(this.currentPath);
      if (p !== this.currentPath) this.navigateTo(p);
    }
  }

  refresh(): void { this.refreshClicked.emit(); }

  async copyPath(): Promise<void> {
    await navigator.clipboard.writeText(this.currentPath).catch(() => {});
    clearTimeout(this.copiedTimer);
    this.justCopied = true;
    this.copiedTimer = setTimeout(() => { this.justCopied = false; this.cdr.detectChanges(); }, 1800);
  }

  navigateTo(p: string): void {
    const np = normalizePath(p);
    this.closeAll();
    if (np === this.currentPath) return;

    const { archiveInnerParts } = parsePathWithArchives(np);
    const pathIsArchive = archiveInnerParts.length > 0 || isArchive(np.split('/').pop() ?? '');

    if (pathIsArchive) {
      this.archiveSegmentClick.emit(np);
      this.currentPath = np;
      this.rebuildSegments();
      return;
    }

    this.currentPath = np;
    this.rebuildSegments();
    this.pathChange.emit(np);
  }

  // ── Close all overlays ────────────────────────────────────────────────────

  private closeAll(): void {
    this.chevIdx       = -1;
    this.sibOpen       = false;
    this.siblings      = [];
    this.showPinDrop   = false;
    this.showDriveMenu = false;
    this.closeCtx();
  }

  // ── Outside-click / keyboard ──────────────────────────────────────────────

  private onEditModeOutsideClick = (e: MouseEvent): void => {
    if (this.editMode && !this.elRef.nativeElement.contains(e.target as Node)) {
      this.exitEditMode(false);
    }
  };

  onHostKeyDown(e: KeyboardEvent): void {
    if ((e.ctrlKey || e.metaKey) && e.key === 'l') { e.preventDefault(); this.enterEditMode(); }
    if (e.altKey && e.key === 'd')                  { e.preventDefault(); this.enterEditMode(); }
  }

  @HostListener('document:keydown', ['$event'])
  onDocKey(e: KeyboardEvent): void {
    if (e.key !== 'Escape') return;
    if (this.ctxMenu.visible)                                             { this.closeCtx();         e.preventDefault(); }
    else if (this.editMode)                                               { this.exitEditMode(false); e.preventDefault(); }
    else if (this.chevIdx >= 0 || this.showPinDrop || this.showDriveMenu) { this.closeAll();          e.preventDefault(); }
  }
}



