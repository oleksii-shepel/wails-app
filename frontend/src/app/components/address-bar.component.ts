import {
  Component, Input, Output, EventEmitter,
  OnInit, OnDestroy, OnChanges, SimpleChanges,
  ViewChild, ElementRef, ChangeDetectorRef,
  HostListener,
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { Subject, debounceTime, distinctUntilChanged, takeUntil } from 'rxjs';
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
    (keydown)="onEditKeyDown($event)"
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

      svg { width: 16px; height: 16px; fill: currentColor; opacity: 0.9; transition: opacity var(--vsc-transition-fast); }
      &:hover svg { opacity: 1; }
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
      scroll-behavior: smooth;
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
      max-width: 180px; overflow: hidden;

      &:hover { background: var(--vsc-list-hover-background); color: var(--vsc-foreground); }
      
      &.ab-seg--root { color: var(--vsc-foreground); font-weight: 500; }
      
      /* Last segment: clicking enters edit mode rather than navigating */
      &.ab-seg--last {
        color: var(--vsc-foreground-bright); cursor: text; max-width: 240px;
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
      background: color-mix(in srgb, #007fd4 20%, transparent);
      border: 1px solid rgba(0,127,212,0.4); border-radius: 4px;
      color: #7ec8f8; font-family: var(--vsc-font-family-mono); font-weight: 600;
      padding: 0 var(--vsc-padding-lg) !important; margin-right: var(--vsc-padding-md) !important;
      cursor: pointer; transition: background 0.08s, border-color 0.08s;
      &:hover { background: color-mix(in srgb, #007fd4 30%, transparent); border-color: rgba(0,127,212,0.6); }
      &.ab-seg--last { background: color-mix(in srgb, #007fd4 18%, transparent); cursor: text; }
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

    /* Compact mode */
    :host-context(.compact-mode) {
      .ab-bar { height: 28px; }
      .ab-btn { width: 22px; height: 22px; svg { width: 14px; height: 14px; } }
      .ab-chev { width: 18px; height: 22px; }
      .ab-seg { height: 22px; line-height: 22px; font-size: var(--vsc-font-size-small); padding: 0 var(--vsc-padding-sm); }
    }

    /* Light theme adjustments */
    :host-context(.vscode-light) {
      .ab-drive-letter { background: color-mix(in srgb, #0060c0 15%, transparent); border-color: rgba(0,96,192,0.4); color: #0060c0; }
      
      .ab-seg--archive-root {
        background: color-mix(in srgb, #ff8c00 12%, transparent);
        border-color: rgba(255, 140, 0, 0.3);
        color: #b85e00;
        
        &.ab-seg--archive-inner {
          border-left-color: #ff8c00;
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
  /** Changes whenever the parent switches tabs — triggers a full state reset
   *  so the address bar reflects the new tab's path cleanly. */
  @Input() tabId = '';

  @Output() pathChange          = new EventEmitter<string>();
  @Output() navigateUpClicked   = new EventEmitter<void>();
  @Output() refreshClicked      = new EventEmitter<void>();
  @Output() openInOtherPane     = new EventEmitter<string>();
  @Output() archiveSegmentClick = new EventEmitter<string>();

  // ── Path state ───────────────────────────────────────────────────────
  currentPath = '/';
  segments: PathSegment[] = [];
  get isWin(): boolean { return isWindowsPath(this.currentPath); }

  // ── Drive switcher ───────────────────────────────────────────────────
  drives: DriveInfo[] = [];
  showDriveMenu = false;
  driveMenuLeft = 0;
  driveMenuTop = 0;
  drivesLoading = false;

  // ── Chevron / sibling dropdown ───────────────────────────────────────
  chevIdx = -1;
  sibOpen  = false; // mirrors chevIdx >= 0 for appDropdown binding
  siblings: SiblingEntry[] = [];
  sibLoading = false;
  sibDropLeft = 0;
  sibDropTop = 0;

  // ── Pinned paths ─────────────────────────────────────────────────────
  pinnedPaths: PinnedPath[] = [];
  showPinDrop = false;
  get isPinned(): boolean { return this.pinnedPaths.some(p => p.path === this.currentPath); }

  // ── Edit mode ────────────────────────────────────────────────────────
  editMode = false;
  editValue = '';

  // ── Autocomplete ─────────────────────────────────────────────────────
  suggestions: PathSuggestion[] = [];
  acIdx = -1;

  // ── Context menu ─────────────────────────────────────────────────────
  ctxMenu: ContextMenuState = { ...CTX_MENU_CLOSED };

  // ── Misc ─────────────────────────────────────────────────────────────
  justCopied = false;
  compact = false;

  private destroy$    = new Subject<void>();
  private editChange$ = new Subject<string>();
  private blurTimer: any;
  private copiedTimer: any;
  private shouldScrollRight = false;
  private wheelListener?: (e: WheelEvent) => void;

  constructor(
    private pathHistoryService: PathHistoryService,
    private apiService: ApiService,
    private archiveService: ArchiveService,
    private elRef: ElementRef,
    private cdr: ChangeDetectorRef,
  ) {}

  // ── Lifecycle ────────────────────────────────────────────────────────

  ngOnInit(): void {
    this.currentPath = normalizePath(this.initialPath);
    this.rebuildSegments();
    this.loadPins();

    this.editChange$
      .pipe(debounceTime(240), distinctUntilChanged(), takeUntil(this.destroy$))
      .subscribe(v => this.fetchSuggestions(v));

    if (this.isWin) this.loadDrives();

    // Handle editMode outside-click (DropdownService handles dropdown clicks)
    document.addEventListener('mousedown', this.onEditModeOutsideClick, { capture: true });
  }

  ngAfterViewInit(): void {
    this.setupWheelScroll();
  }

  ngAfterViewChecked(): void {
    if (this.shouldScrollRight && this.crumbsEl) {
      const el = this.crumbsEl.nativeElement;
      el.scrollLeft = el.scrollWidth;
      this.shouldScrollRight = false;
    }
  }

  private onEditModeOutsideClick = (e: MouseEvent): void => {
    if (this.editMode && !this.elRef.nativeElement.contains(e.target as Node)) {
      this.exitEditMode(false);
    }
  };

  ngOnChanges(changes: SimpleChanges): void {
    if (changes['tabId'] && !changes['tabId'].firstChange) {
      this.closeAll();
      if (this.editMode) {
        clearTimeout(this.blurTimer);
        this.editMode = false;
        this.editValue = '';
        this.suggestions = [];
        this.acIdx = -1;
      }
      const rawNext = changes['initialPath']?.currentValue ?? this.initialPath;
      this.currentPath = normalizePath(rawNext);
      this.rebuildSegments();  // archiveStack already updated in this cycle
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
    this.destroy$.next();
    this.destroy$.complete();
    document.removeEventListener('mousedown', this.onEditModeOutsideClick, { capture: true } as any);
    // Clean up wheel listener
    const el = this.crumbsEl?.nativeElement;
    if (el && this.wheelListener) {
      el.removeEventListener('wheel', this.wheelListener);
    }
  }

  // ── Segments ─────────────────────────────────────────────────────────

  private rebuildSegments(): void {
    const { fsDirPath, archiveInnerParts } = parsePathWithArchives(this.currentPath);

    // Build base segments normally
    this.segments = archiveInnerParts.length > 0
      ? buildSegmentsFromParsedPath(fsDirPath, archiveInnerParts)
      : buildSegments(this.currentPath);

    let insideArchive = false;

    for (let i = 0; i < this.segments.length; i++) {
      const seg = this.segments[i];
      const isRealArchive = isArchive(seg.label);

      this.segments[i] = {
        ...seg,
        isArchiveRoot: isRealArchive,
        isArchiveInner: insideArchive
      };

      // once we hit an archive file, everything after it is inside archive
      if (isRealArchive) insideArchive = true;
    }

    this.shouldScrollRight = true;
  }

  // ── Click handlers ───────────────────────────────────────────────────

  /**
   * Regular FS segment click.
   * • last segment  → enter edit mode; do NOT stopPropagation so dblclick still bubbles.
   * • non-last      → navigate or open in other pane (Ctrl+click); stopPropagation only here.
   */
  onSegClick(seg: PathSegment, e: MouseEvent): void {
    e.stopPropagation();
    this.closeAll();

    if (seg.isArchiveRoot || seg.isArchiveInner) {
      // Delegate to archive click handler
      if (seg.fullPath !== this.currentPath) {
        this.archiveSegmentClick.emit(seg.fullPath);
      } else {
        this.enterEditMode();
      }
      return;
    }

    if (seg.fullPath !== this.currentPath) {
      this.navigateTo(seg.fullPath);
    } else {
      this.enterEditMode();
    }

    .ab-seg-label {
      display: inline-flex;
      align-items: center;
      min-width: 0;
      gap: 2px;
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
      font-size: 0.85em;
      opacity: 0.8;
    }
  }

  getSegmentLabelParts(seg: PathSegment): { main: string; ext?: string } {
    if (!seg) return { main: '' };
    const cleaned = (seg.fullPath || seg.label || '').replace(/\/+$/, '');
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

  /**
   * Drive letter click.
   * • last segment (e.g. "C:" with no sub-path) → enter edit mode.
   * • non-last → open drive switcher dropdown.
   * No stopPropagation on last so dblclick still works.
   */
  onDriveLetterClick(seg: PathSegment, isLast: boolean, e: MouseEvent): void {
    if (isLast) { this.enterEditMode(); return; }
    e.stopPropagation();
    if (this.showDriveMenu) { this.showDriveMenu = false; return; }
    this.closeAll();
    const rect = (e.target as HTMLElement).getBoundingClientRect();
    this.driveMenuLeft = rect.left;
    this.driveMenuTop  = rect.bottom + 4;
    this.showDriveMenu = true;
    if (this.drives.length === 0 && !this.drivesLoading) this.loadDrives();
    // showDriveMenu bound via [(isOpen)] on appDropdown — no manual registration needed
  }
  /**
   * Click on the .ab-crumbs or .ab-crumbs-inner wrapper (empty space) → edit mode.
   */
  onCrumbsClick(e: MouseEvent): void {
    const target = e.target as HTMLElement;
    if (target.classList.contains('ab-crumbs') || 
        target.classList.contains('ab-crumbs-inner')) {
      e.stopPropagation();
      this.enterEditMode();
    }
  }

  private setupWheelScroll(): void {
    const el = this.crumbsEl?.nativeElement;
    if (!el) return;
    this.wheelListener = (e: WheelEvent) => {
      e.preventDefault();
      el.scrollLeft += e.deltaY || e.deltaX;
    };
    el.addEventListener('wheel', this.wheelListener, { passive: false });
  }
  // ── Drive handling ───────────────────────────────────────────────────

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
    try {
      this.drives = await this.apiService.listDrives() || [];
    } catch { this.drives = []; }
    finally { this.drivesLoading = false; this.cdr.detectChanges(); }
  }

  // ── Chevron / siblings ───────────────────────────────────────────────

  onArchiveChevClick(idx: number, seg: PathSegment, e: MouseEvent): void {
    this.toggleChev(idx, seg, e);
  }

  async toggleChev(idx: number, seg: PathSegment, e: MouseEvent): Promise<void> {
    e.stopPropagation();
    this.showPinDrop  = false;
    this.showDriveMenu = false;

    if (this.chevIdx === idx) { this.chevIdx = -1; this.sibOpen = false; this.siblings = []; return; }

    const rect = (e.target as HTMLElement).getBoundingClientRect();
    this.sibDropLeft = Math.round(rect.left);
    this.sibDropTop  = Math.round(rect.bottom + 4);
    this.chevIdx     = idx;
    this.siblings    = [];
    this.sibLoading  = true;

    const activeChildPath = idx + 1 < this.segments.length
      ? this.segments[idx + 1].fullPath
      : null;

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
      // Register once; getter resolves live element at event time
      this.sibOpen = true;
    }
  }

  private async loadArchiveSiblings(
    seg: PathSegment,
    activeChildPath: string | null,
  ): Promise<void> {
    const { fsDirPath, archiveInnerParts } = parsePathWithArchives(this.currentPath);
    if (!archiveInnerParts.length) { this.sibLoading = false; return; }

    // Build a minimal stack from the parsed path so resolveBrowseTarget can work
    // without needing live ArchiveFrame objects with listing data.
    const syntheticStack: ArchiveFrame[] = [{
      fsPath: fsDirPath,
      nestedChain: [],
      innerPath: archiveInnerParts.join('/'),
      listing: { archivePath: fsDirPath, innerPath: archiveInnerParts.join('/'), entries: [] },
    }];

    const target = this.archiveService.resolveBrowseTarget(
      { name: seg.label, fullPath: seg.fullPath },
      syntheticStack,
    );
    if (!target) { this.sibLoading = false; return; }

    try {
      const listing = await this.archiveService.navigateInto(
        { ...target.frame, nestedChain: target.browseChain },
        target.browseInnerPath,
      );
      const activeChildVirtual = activeChildPath?.replace(/\\/g, '/') ?? null;
      const fsPath = fsDirPath.replace(/\\/g, '/');

      this.siblings = listing.listing.entries
        .filter(e =>
          e.type === FileType.DIRECTORY ||
          (e.type === FileType.ARCHIVE) ||
          isArchive(e.name)
        )
        .map(e => {
          const sibInnerPath   = target.browseInnerPath ? `${target.browseInnerPath}/${e.name}` : e.name;
          const chainPart      = target.browseChain.length ? `/${target.browseChain.join('/')}` : '';
          const virtualSibPath = `${fsPath}${chainPart}/${sibInnerPath}`.replace(/\/+/g, '/');
          return {
            name:      e.name,
            path:      virtualSibPath,
            isArchive: e.type === FileType.ARCHIVE || isArchive(e.name),
            isCurrent: activeChildVirtual
              ? activeChildVirtual === virtualSibPath ||
                activeChildVirtual.startsWith(virtualSibPath + '/') ||
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
    if (archiveInnerParts.length > 0) {
      this.archiveSegmentClick.emit(s.path);
    } else {
      this.navigateTo(s.path);
    }
  }

  // ── Pins ─────────────────────────────────────────────────────────────

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

  onSibOpenChange(open: boolean): void {
    this.sibOpen = open;
    if (!open) { this.chevIdx = -1; this.siblings = []; }
  }

  togglePinDrop(): void {
    this.showPinDrop = !this.showPinDrop;
    if (this.showPinDrop) {
      this.chevIdx = -1;
      this.showDriveMenu = false;
    }
  }

  removePin(p: PinnedPath, e: MouseEvent): void {
    e.stopPropagation();
    this.pinnedPaths = this.pinnedPaths.filter(x => x.path !== p.path);
    this.savePins();
  }

  isSegmentPinned(seg: PathSegment | null): boolean {
    return !!seg && this.pinnedPaths.some(p => p.path === seg.fullPath);
  }

  // ── Edit mode ────────────────────────────────────────────────────────

  enterEditMode(): void {
    this.closeAll();
    this.editMode  = true;
    this.editValue = this.currentPath;
    this.suggestions = [];
    this.acIdx = -1;
    this.cdr.detectChanges();
  }

  exitEditMode(commit: boolean): void {
    clearTimeout(this.blurTimer);
    const val = normalizePath(this.editValue);
    this.editMode    = false;
    this.editValue   = '';
    this.suggestions = [];
    this.acIdx = -1;
    if (commit && val && val !== this.currentPath) this.navigateTo(val);
  }

  onEditBlur(): void {
    this.blurTimer = setTimeout(() => this.exitEditMode(false), 170);
  }

  onEditChange(v: string): void {
    this.editValue = v;
    this.editChange$.next(normalizePath(v));
  }

  onEditKeyDown(e: KeyboardEvent): void {
    switch (e.key) {
      case 'Enter':
        e.preventDefault();
        if (this.acIdx >= 0 && this.suggestions[this.acIdx]) {
          this.acceptSuggestion(this.suggestions[this.acIdx]);
        } else {
          this.exitEditMode(true);
        }
        break;
      case 'Escape':    e.preventDefault(); this.exitEditMode(false); break;
      case 'ArrowDown': e.preventDefault(); this.acIdx = Math.min(this.acIdx + 1, this.suggestions.length - 1); break;
      case 'ArrowUp':   e.preventDefault(); this.acIdx = Math.max(this.acIdx - 1, -1); break;
      case 'Tab':       e.preventDefault(); if (this.suggestions.length) this.acceptSuggestion(this.suggestions[0]); break;
    }
  }

  /**
   * Component-level keydown — receives Ctrl+L and Alt+D because
   * ShortcutRegistryService registers both with allowBubble:true,
   * so the global listener calls preventDefault() but NOT stopPropagation().
   */
  onHostKeyDown(e: KeyboardEvent): void {
    if ((e.ctrlKey || e.metaKey) && e.key === 'l') { e.preventDefault(); this.enterEditMode(); }
    if (e.altKey && e.key === 'd') { e.preventDefault(); this.enterEditMode(); }
  }

  // ── Autocomplete ─────────────────────────────────────────────────────

  private async fetchSuggestions(path: string): Promise<void> {
    if (!path?.trim()) { this.suggestions = []; return; }
    try {
      this.suggestions = await this.pathHistoryService.getSuggestions(path, this.currentPath);
      this.acIdx = -1;
      this.cdr.detectChanges();
    } catch { this.suggestions = []; }
  }

  acceptSuggestion(s: PathSuggestion, e?: MouseEvent): void {
    e?.preventDefault();
    clearTimeout(this.blurTimer);
    this.editMode    = false;
    this.editValue   = '';
    this.suggestions = [];
    this.navigateTo(normalizePath(s.path));
  }

  // ── Context menu ─────────────────────────────────────────────────────

  onSegRightClick(seg: PathSegment, idx: number, e: MouseEvent): void {
    e.preventDefault(); e.stopPropagation();
    this.closeAll();
    this.ctxMenu = { visible: true, x: e.clientX, y: e.clientY, segmentIndex: idx, segment: seg };
  }

  ctxNavigateTo(): void  { if (this.ctxMenu.segment) this.navigateTo(this.ctxMenu.segment.fullPath); this.closeCtx(); }
  async ctxCopyPath(): Promise<void>  { if (this.ctxMenu.segment) await navigator.clipboard.writeText(this.ctxMenu.segment.fullPath).catch(() => {}); this.closeCtx(); }
  async ctxCopyName(): Promise<void>  { if (this.ctxMenu.segment) await navigator.clipboard.writeText(this.ctxMenu.segment.label).catch(() => {}); this.closeCtx(); }
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

  // ── Actions ──────────────────────────────────────────────────────────

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
    const pathIsArchive = archiveInnerParts.length > 0
      || isArchive(np.split('/').pop() ?? '');

    if (pathIsArchive) {
      // Path targets an archive or is an archive file itself — let the parent handle it
      this.archiveSegmentClick.emit(np);
      // Update local display immediately for responsiveness
      this.currentPath = np;
      this.rebuildSegments();
      return;
    }

    this.currentPath = np;
    this.rebuildSegments();
    this.pathChange.emit(np);
  }

  // ── Close all overlays ───────────────────────────────────────────────

  private closeAll(): void {
    this.chevIdx      = -1;
    this.sibOpen      = false;
    this.siblings     = [];
    this.showPinDrop  = false;
    this.showDriveMenu = false;
    this.closeCtx();
  }

  // ── Outside click / escape ───────────────────────────────────────────
  // Note: outside-click closing is now handled by DropdownService for all
  // registered dropdowns. The editMode outside-click is handled below.

  @HostListener('document:keydown', ['$event'])
  onDocKey(e: KeyboardEvent): void {
    if (e.key === 'Escape') {
      if (this.ctxMenu.visible)                                              { this.closeCtx(); e.preventDefault(); }
      else if (this.editMode)                                                { this.exitEditMode(false); e.preventDefault(); }
      else if (this.chevIdx >= 0 || this.showPinDrop || this.showDriveMenu) { this.closeAll(); e.preventDefault(); }
    }
  }
}
