import {
  Component, Input, Output, EventEmitter,
  OnInit, OnDestroy, OnChanges, SimpleChanges,
  ViewChild, ElementRef, ChangeDetectorRef,
  HostListener,
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { DropdownTriggerDirective, DropdownMenuDirective } from '../directives/dropdown.directive';

import { ApiService } from '../services/api.service';
import { DirectorySourceService } from '../services/directory-source.service';
import { DirectoryListing, DriveInfo, SchemeType, FileInfo, FileType, SpecialFolder } from '../shared/protocol';
import { ArchiveService, ArchiveFrame, isArchive } from '../services/archive.service';
import { parsePathWithArchives } from './address-bar.utils';
import { EventsOn } from 'wailsjs/runtime';

import { PathSegment, SiblingEntry, PinnedPath, ContextMenuState, PINNED_KEY, CTX_MENU_CLOSED } from './address-bar.types';
import { buildSegments, isWindowsPath, normalizePath, pathIdentityKey, samePath, shortenPath } from './address-bar.utils';
import { AddressBarEditComponent } from './address-bar-edit.component';
import { AddressBarSchemeDropdownComponent } from './address-bar-scheme-dropdown.component';
import { AddressBarSiblingDropdownComponent } from './address-bar-sibling-dropdown.component';
import { AddressBarQuickAccessComponent } from './address-bar-quick-access.component';
import { DropdownContainerComponent } from './dropdown-container.component';

@Component({
  selector: 'app-address-bar',
  standalone: true,
  imports: [
    CommonModule,
    DropdownContainerComponent,
    DropdownTriggerDirective,
    DropdownMenuDirective,
    AddressBarSiblingDropdownComponent,
    AddressBarSchemeDropdownComponent,
    AddressBarEditComponent,
    AddressBarQuickAccessComponent,
  ],
  template: `
<div class="ab-host" (keydown)="onHostKeyDown($event)">

  <!-- ── BREADCRUMB ROW ──────────────────────────────────────────────── -->
  <div class="ab-bar" *ngIf="!editMode" [class.ab-bar--compact]="compact">

    <!-- Left cluster: quick access + pin star -->
    <div class="ab-cluster ab-cluster--left">
      <!-- QUICK ACCESS -->
      <button
        #quickAccessTrigger
        appDropdownTrigger
        class="ab-btn ab-btn--quick-access"
        [class.ab-btn--open]="showQuickAccess"
        (click)="toggleQuickAccess()"
        title="Quick Access (Ctrl+Q)"
        aria-label="Open Quick Access"
      >
        <svg width="13" height="13" viewBox="0 0 16 16" fill="currentColor">
          <path d="M2 3.5a.5.5 0 0 1 .5-.5h11a.5.5 0 0 1 0 1h-11a.5.5 0 0 1-.5-.5zM2 8a.5.5 0 0 1 .5-.5h11a.5.5 0 0 1 0 1h-11A.5.5 0 0 1 2 8zm0 4.5a.5.5 0 0 1 .5-.5h6a.5.5 0 0 1 0 1h-6a.5.5 0 0 1-.5-.5z"/>
        </svg>
      </button>
      <app-dropdown-container
        class="ab-drop-anchor"
        [(isOpen)]="showQuickAccess"
        [anchorEl]="quickAccessTrigger"
      >
        <app-address-bar-quick-access
          appDropdownMenu
          *ngIf="showQuickAccess"
          [currentPath]="currentPath"
          [specialFolders]="specialFolders"
          [pinnedPaths]="pinnedPaths"
          [preferredSectionTitle]="quickAccessPreferredSection"
          (navigate)="navigateTo($event); showQuickAccess = false"
          (removePin)="removePin($event)"
          (closeRequested)="showQuickAccess = false"
        />
      </app-dropdown-container>

      <button
        class="ab-btn ab-btn--star"
        [class.ab-btn--pinned]="isPinned"
        (click)="togglePin()"
        [title]="isPinned ? 'Unpin this path' : 'Pin this path'"
        aria-label="Toggle pin"
      >{{ isPinned ? '★' : '☆' }}</button>
    </div>

    <!-- Breadcrumb trail -->
    <div class="ab-crumbs" (click)="onCrumbsClick($event)">
      <!-- Scheme root stays fixed, outside the scroll -->
      <ng-container *ngIf="(isWin || isSchemeRoot) && segments.length > 0">
        <span
          class="ab-seg ab-scheme"
          [class.ab-seg--last]="segments.length === 1"
          [class.ab-scheme--synthetic]="isSchemeRoot || isCurrentRootSynthetic()"
          [class.ab-scheme--open]="showSchemeMenu"
          (click)="onSchemeClick(segments[0], segments.length === 1, $event)"
          (contextmenu)="onSegRightClick(segments[0], 0, $event)"
          title="Switch schemes"
        >
          <span class="ab-scheme-text">{{ currentSchemeLabel() }}</span>
          <span *ngIf="showCurrentSchemeSeparator()" class="ab-scheme-separator">:</span>
        </span>

        <div class="ab-scheme-chevron-wrap" *ngIf="segments.length > 1">
          <button
            class="ab-chev ab-scheme-chevron"
            [class.ab-chev--open]="chevIdx === 0"
            (click)="toggleChev(0, segments[0], $event)"
            [title]="'Show contents of ' + segments[0].fullPath"
            aria-label="Show root contents"
          ><svg viewBox="0 0 16 16"><path d="M6 4l4 4-4 4" fill="none" stroke="currentColor" stroke-width="2"/></svg></button>
        </div>
      </ng-container>

      <!-- Everything else scrolls -->
      <div class="ab-crumbs-inner" #crumbsEl>
        <ng-container *ngFor="let seg of segments; let i = index; let last = last">
          <!-- skip index 0 on Windows or scheme roots (rendered above as fixed badge) -->
          <ng-container *ngIf="(!isWin && !isSchemeRoot) || i > 0">

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
              ><svg viewBox="0 0 16 16"><path d="M6 4l4 4-4 4" fill="none" stroke="currentColor" stroke-width="2"/></svg></button>
            </div>

          </ng-container>
        </ng-container>
      </div>
      <span class="ab-crumbs-filler" aria-hidden="true"></span>
      <span
        class="ab-status"
        [class.ab-status--visible]="loading || showLoadComplete"
        [attr.aria-hidden]="!(loading || showLoadComplete)"
        [attr.aria-label]="loading ? 'Loading' : (showLoadComplete ? 'Load complete' : null)"
        [attr.title]="loading ? 'Loading' : (showLoadComplete ? 'Load complete' : null)">
        <span *ngIf="loading" class="ab-loading-spinner"></span>
        <svg *ngIf="!loading && showLoadComplete" class="ab-status-check" viewBox="0 0 16 16" fill="none" aria-hidden="true">
          <path d="M3 8.5 L6.2 11.5 L13 4.5" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
        </svg>
      </span>
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
    [currentPath]="currentPath"
    [placeholder]="placeholder"
    [focusOnInit]="editMode"
    (commit)="onEditCommit($event)"
    (cancel)="exitEditMode(false)"
  />

  <!-- ── SIBLING DROPDOWN ──────────────────────────────────────────────── -->
  <app-dropdown-container
    [(isOpen)]="sibOpen"
    (isOpenChange)="onSibOpenChange($event)"
    [anchorEl]="sibAnchorEl"
    *ngIf="chevIdx >= 0"
  >
    <app-address-bar-sibling-dropdown
      appDropdownMenu
      [entries]="siblings"
      [loading]="sibLoading"
      [headLabel]="segments[chevIdx].label"
      (entryClick)="onSibClick($event)"
      (closeRequested)="onSibOpenChange(false)"
    />
  </app-dropdown-container>

  <!-- ── SCHEME DROPDOWN ────────────────────────────────────────────────── -->
  <app-dropdown-container
    [(isOpen)]="showSchemeMenu"
    [anchorEl]="schemeAnchorEl"
    *ngIf="showSchemeMenu"
  >
    <app-address-bar-scheme-dropdown
      appDropdownMenu
      [schemes]="schemes"
      [loading]="schemesLoading"
      [currentPath]="currentPath"
      (schemeClick)="switchToScheme($event)"
      (closeRequested)="showSchemeMenu = false"
    />
  </app-dropdown-container>

  <!-- ── CONTEXT MENU ──────────────────────────────────────────────────── -->
  <app-dropdown-container
    [(isOpen)]="ctxMenu.visible"
    (isOpenChange)="onCtxMenuOpenChange($event)"
    [anchorEl]="ctxAnchorEl"
  >
    <div
      class="ab-ctx-menu"
      appDropdownMenu
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
  </app-dropdown-container>

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
      border-bottom: 1px solid var(--vsc-border);  /* ← moved here */
      box-sizing: border-box;
    }

    .ab-host {
      position: relative;
      background: var(--vsc-sidebar-background);
      height: 32px;
      display: flex;
      align-items: center;
      overflow: visible;
      box-sizing: border-box;
    }

    .ab-bar {
      /* use grid so the middle (breadcrumbs) reliably fills remaining space */
      display: grid;
      grid-template-columns: auto 1fr auto;
      align-items: center;
      /* reduced height */
      height: 32px;
      padding: 0 var(--vsc-padding-sm);
      gap: var(--vsc-padding-xs);
      width: 100%;
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

    .ab-status {
      width: 20px;
      height: 20px;
      display: flex;
      align-items: center;
      justify-content: center;
      flex-shrink: 0;
      color: var(--vsc-accent-blue);
      margin-left: var(--vsc-padding-xs);
      opacity: 0;
      visibility: hidden;
      pointer-events: none;
      transition: color var(--vsc-transition-fast), opacity var(--vsc-transition-fast);
    }

    .ab-status--visible {
      opacity: 1;
      visibility: visible;
    }

    .ab-loading-spinner {
      width: 12px;
      height: 12px;
      border-radius: 999px;
      border: 2px solid color-mix(in srgb, var(--vsc-accent-blue) 22%, transparent);
      border-top-color: var(--vsc-accent-blue);
      box-sizing: border-box;
      animation: ab-spin 0.9s linear infinite;
    }

    .ab-status-check {
      width: 12px;
      height: 12px;
      color: var(--vsc-success, #4caf50);
      animation: ab-status-pop 160ms ease-out;
    }

    @keyframes ab-spin {
      from { transform: rotate(0deg); }
      to { transform: rotate(360deg); }
    }

    @keyframes ab-status-pop {
      from { transform: scale(0.82); opacity: 0.45; }
      to { transform: scale(1); opacity: 1; }
    }

    .ab-btn {
      display: flex; align-items: center; justify-content: center;
      width: 22px; height: 22px; padding: 0;
      background: transparent; border: none;
      border-radius: var(--vsc-border-radius-sm);
      color: var(--vsc-foreground-dim); cursor: pointer;
      transition: background-color var(--vsc-transition-fast), color var(--vsc-transition-fast);
      flex-shrink: 0;

      &:hover { background: var(--vsc-list-hover-background); color: var(--vsc-foreground-bright); }
      &:active { background: var(--vsc-list-active-background); color: var(--vsc-foreground-bright); }
      &:focus-visible { outline: 1px solid var(--vsc-focusBorder, var(--vsc-accent-blue)); outline-offset: -1px; }

      svg { width: 14px; height: 14px; fill: currentColor; opacity: 0.9; transition: opacity var(--vsc-transition-fast), filter 120ms ease; }
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
      /* take all remaining space between left and right clusters */
      flex: 1 1 auto;
      min-width: 0;
      display: flex;
      align-items: center;
      overflow: hidden;
      gap: 0;
    }

    .ab-crumbs-inner {
      /* size to breadcrumb content and scroll only when content exceeds space */
      flex: 0 1 auto;
      min-width: 0;
      max-width: 100%;
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

    .ab-crumbs-filler {
      flex: 1 1 auto;
      min-width: 12px;
      height: 100%;
    }

    .ab-scheme {
      flex-shrink: 0;
    }

    .ab-seg {
      display: inline-flex; align-items: center;
      height: 20px; padding: 0 var(--vsc-padding-sm);
      border-radius: var(--vsc-border-radius-sm);
      color: var(--vsc-foreground-dim); font-size: var(--vsc-font-size);
      line-height: 20px; cursor: pointer;
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

    .ab-scheme {
      /* Softer scheme styling: lower opacity and subtler border */
      background: color-mix(in srgb, #007fd4 12%, transparent);
      border: 1px solid rgba(0,127,212,0.25);
      border-radius: 4px;
      color: #6fb3db; /* slightly dimmer blue */
      font-family: var(--vsc-font-family-mono); font-weight: 600;
      padding: 0 var(--vsc-padding-lg) !important; margin-right: var(--vsc-padding-md) !important;
      cursor: pointer; transition: background 0.12s, border-color 0.12s, color 120ms;
      &:hover { background: color-mix(in srgb, #007fd4 20%, transparent); border-color: rgba(0,127,212,0.38); }
      &.ab-seg--last { background: color-mix(in srgb, #007fd4 10%, transparent); cursor: pointer; }
      &.ab-scheme--open { background: color-mix(in srgb, #007fd4 22%, transparent); border-color: rgba(0,127,212,0.45); }
      &.ab-scheme--synthetic {
        gap: 0;
        max-width: 180px;
      }
    }

    .ab-scheme-text {
      display: block;
      flex: 0 1 auto;
      min-width: 0;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
      padding-right: 0;
    }

    .ab-scheme-separator {
      flex: 0 0 auto;
      margin-left: 0.12em;
      padding-right: 0;
      color: inherit;
      line-height: 1;
    }


    .ab-chev-wrap {
      position: relative; flex-shrink: 0; overflow: visible;
      display: inline-flex; align-items: center;
    }

    .ab-chev {
      display: flex; align-items: center; justify-content: center;
      width: 20px; height: 24px; padding: 0;
      background: transparent !important; border: none;
      color: var(--vsc-foreground-dim);
      cursor: pointer; border-radius: var(--vsc-border-radius-sm);
      transition: color var(--vsc-transition-fast), transform var(--vsc-transition-normal);

      svg { width: 12px; height: 12px; flex-shrink: 0; }

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

    .ab-scheme-chevron-wrap { display: inline-flex; align-items: center; margin-right: var(--vsc-padding-xs); }
    .ab-scheme-chevron {
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
      .ab-scheme { background: color-mix(in srgb, #0060c0 10%, transparent); border-color: rgba(0,96,192,0.28); color: #0a6fb0;
        &.ab-scheme--open { background: color-mix(in srgb, #0060c0 18%, transparent); border-color: rgba(0,96,192,0.42); }
      }
      
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
  @Input() loading = false;
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
  /** True when the first segment is a non-Windows scheme root (memory://, ftp://, Registry, etc.) */
  get isSchemeRoot(): boolean {
    if (this.isWin || this.segments.length === 0) return false;
    const seg = this.segments[0];
    return seg.isRoot === true && seg.label !== '/';
  }

  // ── Scheme switcher ───────────────────────────────────────────────────────
  schemes: DriveInfo[] = [];
  showSchemeMenu = false;
  schemeAnchorEl: HTMLElement | null = null;
  schemesLoading = false;

  // ── Chevron / sibling dropdown ───────────────────────────────────────────
  chevIdx     = -1;
  sibOpen     = false;
  siblings: SiblingEntry[] = [];
  sibLoading  = false;
  sibAnchorEl: HTMLElement | null = null;

  private readonly sibPaneId = 'ab-sib-' + Math.random().toString(36).slice(2, 8);
  private activeSibStreamId: string | null = null;
  private sibStreamOff: Array<() => void> = [];

  // ── Quick Access ─────────────────────────────────────────────────────────
  specialFolders: SpecialFolder[] = [];
  showQuickAccess = false;
  quickAccessPreferredSection: string | null = null;

  // ── Pinned paths ─────────────────────────────────────────────────────────
  pinnedPaths: PinnedPath[] = [];
  get isPinned(): boolean { return this.pinnedPaths.some(p => samePath(p.path, this.currentPath)); }

  // ── Edit mode ────────────────────────────────────────────────────────────
  editMode  = false;

  // ── Context menu ─────────────────────────────────────────────────────────
  ctxMenu: ContextMenuState = { ...CTX_MENU_CLOSED };
  ctxAnchorEl: HTMLElement | null = null;

  // ── Misc ─────────────────────────────────────────────────────────────────
  justCopied = false;
  compact    = false;
  showLoadComplete = false;

  // ── Timers ───────────────────────────────────────────────────────────────
  private copiedTimer: ReturnType<typeof setTimeout> | undefined;
  private loadCompleteTimer: ReturnType<typeof setTimeout> | undefined;

  // ── Scroll ───────────────────────────────────────────────────────────────
  private scrollAnimFrame: number | null = null;
  private wheelListener?: (e: WheelEvent) => void;

  constructor(
    private apiService: ApiService,
    private directorySource: DirectorySourceService,
    private archiveService: ArchiveService,
    private elRef: ElementRef,
    private cdr: ChangeDetectorRef,
  ) {}

  // ── Lifecycle ─────────────────────────────────────────────────────────────

  ngOnInit(): void {
    this.currentPath = normalizePath(this.initialPath);
    this.rebuildSegments();
    this.loadPins();
    this.loadSpecialFolders();
    this.loadSchemes();
  }

  ngAfterViewInit(): void {
    this.setupWheelScroll();
  }

  ngAfterViewChecked(): void {}

  ngOnChanges(changes: SimpleChanges): void {
    if (changes['loading'] && !changes['loading'].firstChange) {
      const previous = !!changes['loading'].previousValue;
      const current = !!changes['loading'].currentValue;
      if (previous && !current) {
        this.showTransientLoadComplete();
      } else if (current) {
        this.clearTransientLoadComplete();
      }
    }

    if (changes['tabId'] && !changes['tabId'].firstChange) {
      this.closeAll();
      if (this.editMode) {
        this.editMode    = false;
      }
      const rawNext = changes['initialPath']?.currentValue ?? this.initialPath;
      this.currentPath = normalizePath(rawNext);
      this.rebuildSegments();
      this.cdr.detectChanges();
      return;
    }

    if (changes['initialPath'] && !changes['initialPath'].firstChange) {
      const next = normalizePath(changes['initialPath'].currentValue);
      if (!samePath(next, this.currentPath)) {
        this.currentPath = next;
        this.rebuildSegments();
      }
    }
  }

  ngOnDestroy(): void {
    clearTimeout(this.copiedTimer);
    this.clearTransientLoadComplete();
    if (this.scrollAnimFrame !== null) cancelAnimationFrame(this.scrollAnimFrame);
    const el = this.crumbsEl?.nativeElement;
    if (el && this.wheelListener) el.removeEventListener('wheel', this.wheelListener);
    this.closeSiblingStream();
  }

  private showTransientLoadComplete(): void {
    this.clearTransientLoadComplete();
    this.showLoadComplete = true;
    this.cdr.markForCheck();
    this.loadCompleteTimer = setTimeout(() => {
      this.showLoadComplete = false;
      this.loadCompleteTimer = undefined;
      this.cdr.markForCheck();
    }, 900);
  }

  private clearTransientLoadComplete(): void {
    if (this.loadCompleteTimer) {
      clearTimeout(this.loadCompleteTimer);
      this.loadCompleteTimer = undefined;
    }
    if (this.showLoadComplete) {
      this.showLoadComplete = false;
      this.cdr.markForCheck();
    }
  }

  // ── Segments ──────────────────────────────────────────────────────────────

  private rebuildSegments(): void {
    this.segments = this.directorySource.buildPathSegments(this.currentPath);
    // Animate scroll to end after segments update (next frame so DOM has rendered)
    requestAnimationFrame(() => this.animateScrollToEnd());
  }

  private currentRootPath(): string {
    const root = this.directorySource.getRootPath(this.currentPath) || this.segments[0]?.fullPath || this.currentPath;
    return normalizePath(root || '');
  }

  private findCurrentRootEntry(): DriveInfo | null {
    const rootPath = this.currentRootPath();
    if (!rootPath) return null;
    return this.schemes.find((drive) => normalizePath(drive.path) === rootPath) ?? null;
  }

  isCurrentRootSynthetic(): boolean {
    const drive = this.findCurrentRootEntry();
    if (!drive) return false;
    return /^(memory|registry|search|smb|ftp|sftp|webdav)$/.test(drive.schemeType);
  }

  isCurrentRootPhysical(): boolean {
    // Draw simple Windows drive letters (e.g. "C:") with blue styling
    const lbl = this.segments[0]?.label || '';
    return this.isWin && /^[A-Za-z]:$/.test(lbl);
  }

  showCurrentSchemeSeparator(): boolean {
    return this.isSchemeRoot || this.isCurrentRootSynthetic() || this.isCurrentRootPhysical();
  }

  currentSchemeLabel(): string {
    const drive = this.findCurrentRootEntry();
    if (this.showCurrentSchemeSeparator()) {
      if (this.isCurrentRootPhysical()) {
        const rawPhysical = this.segments[0]?.label || drive?.name || '';
        return rawPhysical.replace(/\s*:\s*$/, '');
      }

      const path = drive?.path || '';
      const match = /^([a-z][a-z0-9+.-]*):\/\//i.exec(path);
      if (match) return match[1].toLowerCase();

      const raw = this.segments[0]?.label || drive?.name || '';
      return raw.replace(/\s*:\s*$/, '');
    }
    if (!drive) {
      return this.segments[0]?.label || '';
    }
    // For Windows drive, show drive letter (e.g. 'C:')
    return this.segments[0]?.label || drive.name;
  }

  currentSchemeIcon(): string | null {
    const drive = this.findCurrentRootEntry();
    if (!drive || !this.isCurrentRootSynthetic()) return null;

    if (drive.schemeType === 'memory') return '🧠';
    if (drive.schemeType === 'registry') return '🧩';
    if (drive.schemeType === 'search') return '🔎';
    if (drive.schemeType === 'smb') return '🖧';
    if (drive.schemeType === 'ftp') return '📡';
    if (drive.schemeType === 'sftp') return '🔐';
    if (drive.schemeType === 'webdav') return '🌍';
    return null;
  }

  // ── Segment click handlers ────────────────────────────────────────────────

  onSegClick(seg: PathSegment, e: MouseEvent): void {
    e.stopPropagation();
    this.closeAll();
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
    this.navigateTo(seg.fullPath);
  }

  onArchiveRootSegClick(seg: PathSegment, isLast: boolean, e: MouseEvent): void {
    if (isLast) { this.enterEditMode(); return; }
    e.stopPropagation();
    this.closeAll();
    this.navigateTo(seg.fullPath);
  }

  onSchemeClick(seg: PathSegment, isLast: boolean, e: MouseEvent): void {
    // Scheme root always opens the scheme switcher — even when at the root.
    // Unlike regular segments, there is no useful "edit the last crumb" behaviour.
    e.stopPropagation();
    if (this.showSchemeMenu) { this.showSchemeMenu = false; return; }
    this.closeAll();
    this.schemeAnchorEl = e.currentTarget as HTMLElement;
    this.showSchemeMenu = true;
    if (this.schemes.length === 0 && !this.schemesLoading) this.loadSchemes();
  }

  onCrumbsClick(e: MouseEvent): void {
    const target = e.target as HTMLElement | null;
    if (target?.closest('.ab-seg, .ab-chev, .ab-scheme-chevron')) {
      return;
    }

    e.stopPropagation();
    this.enterEditMode();
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

  onCtxMenuOpenChange(open: boolean): void {
    if (!open) {
      this.closeCtx();
    }
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

  // ── Scheme handling ────────────────────────────────────────────────────────

  switchToScheme(drive: DriveInfo): void {
    this.showSchemeMenu = false;
    const raw = (drive.path || drive.name || '').toString();
    const normalized = normalizePath(raw);
    let target = normalized;

    // For UNC, keep/share root form (//server/share).
    if (normalized.startsWith('//')) {
      const parts = normalized.split('/').filter(Boolean);
      target = parts.length >= 2 ? `//${parts[0]}/${parts[1]}` : normalized;
    }

    if (target && !samePath(target, this.currentPath)) this.navigateTo(target);
  }

  private async pathExists(path: string): Promise<boolean> {
    try {
      const info = await this.apiService.getFileInfo(path);
      return info.type === FileType.DIRECTORY;
    } catch {
      return false;
    }
  }

  private getHomeFolderPath(): string | null {
    const home = this.specialFolders.find(f => f.name.toLowerCase() === 'home')?.path;
    return home ? normalizePath(home) : null;
  }

  private async getCloudschemes(): Promise<DriveInfo[]> {
    if (!this.specialFolders.length) {
      await this.loadSpecialFolders();
    }

    const home = this.getHomeFolderPath();
    if (!home) {
      return [];
    }

    const providers: Array<{ name: string; schemeType: SchemeType; paths: string[] }> = [
      {
        name: 'OneDrive',
        schemeType: 'cloud-onedrive',
        paths: [
          `${home}/OneDrive`,
          `${home}/OneDrive - Personal`,
        ],
      },
      {
        name: 'Google Drive',
        schemeType: 'cloud-google',
        paths: [
          `${home}/Google Drive`,
          `${home}/My Drive`,
          `${home}/Google Drive/My Drive`,
        ],
      },
      {
        name: 'Dropbox',
        schemeType: 'cloud-dropbox',
        paths: [
          `${home}/Dropbox`,
        ],
      },
    ];

    const cloudschemes: DriveInfo[] = [];
    for (const provider of providers) {
      let foundPath: string | null = null;
      for (const candidate of provider.paths.map(normalizePath)) {
        if (await this.pathExists(candidate)) {
          foundPath = candidate;
          break;
        }
      }

      if (!foundPath) {
        continue;
      }

      cloudschemes.push({
        name: provider.name,
        path: foundPath,
        schemeType: provider.schemeType,
        totalSpace: 0,
        freeSpace: 0,
        fileSystem: 'Cloud',
      });
    }

    return cloudschemes;
  }

  async loadSchemes(): Promise<void> {
    this.schemesLoading = true;
    try { 
      const schemesList = await this.apiService.getDrives();
      const systemschemes = schemesList?.schemes || [];
      const cloudschemes = await this.getCloudschemes();
      this.schemes = [...systemschemes, ...cloudschemes];
    } catch {
      this.schemes = [];
    } finally { 
      this.schemesLoading = false; 
      this.cdr.detectChanges(); 
    }
  }

  // ── Chevron / siblings ────────────────────────────────────────────────────

  async toggleChev(idx: number, seg: PathSegment, e: MouseEvent): Promise<void> {
    e.stopPropagation();
    this.showSchemeMenu = false;

    if (this.chevIdx === idx) {
      this.closeSiblingStream();
      this.chevIdx = -1; this.sibOpen = false; this.siblings = [];
      return;
    }

    this.closeSiblingStream();
    this.sibAnchorEl = e.currentTarget as HTMLElement;
    this.chevIdx    = idx;
    this.siblings   = [];
    this.sibLoading = true;

    const activeChildPath = idx + 1 < this.segments.length ? this.segments[idx + 1].fullPath : null;

    if (seg.isArchiveRoot || seg.isArchiveInner) {
      try {
        await this.loadArchiveSiblings(seg, activeChildPath);
      } catch {
        this.siblings = [];
        this.sibLoading = false;
        this.cdr.detectChanges();
        this.sibOpen = true;
      }
      return;
    }

    if (!this.directorySource.supportsStreaming(seg.fullPath)) {
      try {
        const listing = await this.directorySource.listDirectory(seg.fullPath, false);
        this.siblings = (listing.entries || [])
          .filter((entry) => entry.type === FileType.DIRECTORY)
          .map((entry) => ({
            name: entry.name,
            path: entry.path,
            isCurrent: samePath(entry.path, activeChildPath),
          }))
          .sort((a, b) => a.name.localeCompare(b.name));
      } catch {
        this.siblings = [];
      } finally {
        this.sibLoading = false;
        this.sibOpen = true;
        this.cdr.detectChanges();
      }
      return;
    }

    // Stream directory contents progressively
    let streamId: string;
    try {
      streamId = await this.apiService.startListDirectoryStream(this.sibPaneId, seg.fullPath, false);
    } catch {
      this.siblings = [];
      this.sibLoading = false;
      this.cdr.detectChanges();
      this.sibOpen = true;
      return;
    }
    this.activeSibStreamId = streamId;

    const offBatch = EventsOn('fs:listStream:batch', (ev: any) => {
      if (ev?.streamId !== streamId) return;
      for (const entry of (ev.entries as FileInfo[] || [])) {
        if (entry.type !== FileType.DIRECTORY) continue;
        this.siblings.push({ name: entry.name, path: entry.path, isCurrent: samePath(entry.path, activeChildPath) });
      }
      this.siblings = this.siblings.slice().sort((a, b) => a.name.localeCompare(b.name));
      if (this.sibLoading) { this.sibLoading = false; this.sibOpen = true; }
      this.cdr.detectChanges();
    });

    const offDone = EventsOn('fs:listStream:done', (ev: any) => {
      if (ev?.streamId !== streamId) return;
      offBatch(); offDone(); offError();
      this.sibStreamOff = [];
      if (this.activeSibStreamId === streamId) this.activeSibStreamId = null;
      this.sibLoading = false;
      if (!this.sibOpen) this.sibOpen = true;
      this.cdr.detectChanges();
    });

    const offError = EventsOn('fs:listStream:error', (ev: any) => {
      if (ev?.streamId !== streamId) return;
      offBatch(); offDone(); offError();
      this.sibStreamOff = [];
      if (this.activeSibStreamId === streamId) this.activeSibStreamId = null;
      this.sibLoading = false;
      if (!this.sibOpen) this.sibOpen = true;
      this.cdr.detectChanges();
    });

    this.sibStreamOff = [offBatch, offDone, offError];
  }

  private async loadArchiveSiblings(seg: PathSegment, activeChildPath: string | null): Promise<void> {
    const { fsDirPath, archiveInnerParts } = parsePathWithArchives(this.currentPath);
    if (!archiveInnerParts.length) { this.sibLoading = false; return; }

    const syntheticStack: ArchiveFrame[] = [{
      fsPath: fsDirPath, nestedChain: [],
      innerPath: archiveInnerParts.join('/'),
      listing: { archivePath: fsDirPath, innerPath: archiveInnerParts.join('/'), entries: [] },
    }];

    const target = this.directorySource.resolveArchiveBrowseTarget(
      { name: seg.label, fullPath: seg.fullPath }, syntheticStack,
    );
    if (!target) { this.sibLoading = false; return; }

    try {
      const listing = await this.directorySource.navigateArchive(
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
    this.navigateTo(s.path);
  }

  onSibOpenChange(open: boolean): void {
    this.sibOpen = open;
    if (!open) {
      this.closeSiblingStream();
      this.chevIdx = -1;
      this.siblings = [];
    }
  }

  private closeSiblingStream(): void {
    this.sibStreamOff.forEach(f => f());
    this.sibStreamOff = [];
    if (this.activeSibStreamId) {
      void this.apiService.cancelListDirectoryStream(this.activeSibStreamId);
      this.activeSibStreamId = null;
    }
  }

  // ── Pins ──────────────────────────────────────────────────────────────────

  private async loadSpecialFolders(): Promise<void> {
    try {
      this.specialFolders = await this.apiService.getSpecialFolders();
    } catch (error) {
      console.error('Failed to load special folders:', error);
      this.specialFolders = [];
    }
  }

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
      const segments = buildSegments(this.currentPath);
      const label = segments[segments.length - 1]?.label || (isWindowsPath(this.currentPath) ? shortenPath(this.currentPath) : '/');
      this.pinnedPaths.push({ path: this.currentPath, label, icon: '📁', addedAt: Date.now() });
    }
    this.savePins();
  }

  toggleQuickAccess(): void {
    this.openQuickAccess(null, !this.showQuickAccess);
  }

  private openQuickAccess(preferredSectionTitle: string | null, toggleWhenSame = false): void {
    const samePreference = this.quickAccessPreferredSection === preferredSectionTitle;
    if (toggleWhenSame && samePreference) {
      this.showQuickAccess = !this.showQuickAccess;
    } else {
      this.showQuickAccess = true;
    }

    this.quickAccessPreferredSection = preferredSectionTitle;
    if (this.showQuickAccess) {
      this.chevIdx = -1;
      this.showSchemeMenu = false;
    }
  }

  removePin(p: PinnedPath, e?: MouseEvent): void {
    e?.stopPropagation();
    this.pinnedPaths = this.pinnedPaths.filter(x => !samePath(x.path, p.path));
    this.savePins();
  }

  isSegmentPinned(seg: PathSegment | null): boolean {
    return !!seg && this.pinnedPaths.some(p => samePath(p.path, seg.fullPath));
  }

  // ── Edit mode ─────────────────────────────────────────────────────────────

  enterEditMode(): void {
    this.closeAll();
    this.editMode    = true;
    this.cdr.detectChanges();
  }

  exitEditMode(commit: boolean): void {
    this.editMode    = false;
    if (commit) {
      // commit is now handled by onEditCommit
    }
  }

  onEditCommit(path: string): void {
    this.exitEditMode(false);
    const np = normalizePath(path);
    if (np && !samePath(np, this.currentPath)) this.navigateTo(np);
  }

  // ── Context menu ──────────────────────────────────────────────────────────

  onSegRightClick(seg: PathSegment, idx: number, e: MouseEvent): void {
    e.preventDefault(); e.stopPropagation();
    this.closeAll();
    this.ctxAnchorEl = e.currentTarget as HTMLElement;
    this.ctxMenu = { visible: true, segmentIndex: idx, segment: seg };
  }

  ctxNavigateTo(): void   { if (this.ctxMenu.segment) this.navigateTo(this.ctxMenu.segment.fullPath); this.closeCtx(); }
  async ctxCopyPath(): Promise<void> { if (this.ctxMenu.segment) await navigator.clipboard.writeText(this.ctxMenu.segment.fullPath).catch(() => {}); this.closeCtx(); }
  async ctxCopyName(): Promise<void> { if (this.ctxMenu.segment) await navigator.clipboard.writeText(this.ctxMenu.segment.label).catch(() => {}); this.closeCtx(); }
  ctxOpenOtherPane(): void { if (this.ctxMenu.segment) this.openInOtherPane.emit(this.ctxMenu.segment.fullPath); this.closeCtx(); }

  ctxPinPath(): void {
    const seg = this.ctxMenu.segment;
    if (!seg) { this.closeCtx(); return; }
    const already = this.pinnedPaths.some(p => samePath(p.path, seg.fullPath));
    if (already) {
      this.pinnedPaths = this.pinnedPaths.filter(p => !samePath(p.path, seg.fullPath));
    } else {
      this.pinnedPaths.push({ path: seg.fullPath, label: seg.label, icon: '📁', addedAt: Date.now() });
    }
    this.savePins();
    this.closeCtx();
  }

  private closeCtx(): void { this.ctxMenu = { ...CTX_MENU_CLOSED }; this.ctxAnchorEl = null; }

  // ── Navigation ────────────────────────────────────────────────────────────

  goUp(): void {
    this.navigateUpClicked.emit();
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
    if (samePath(np, this.currentPath)) return;

    this.currentPath = np;
    this.rebuildSegments();
    this.pathChange.emit(np);
  }

  // ── Close all overlays ────────────────────────────────────────────────────

  private closeAll(): void {
    this.chevIdx       = -1;
    this.sibOpen       = false;
    this.siblings      = [];
    this.showQuickAccess = false;
    this.quickAccessPreferredSection = null;
    this.showSchemeMenu = false;
    this.closeCtx();
  }

  // ── Outside-click / keyboard ──────────────────────────────────────────────

  onHostKeyDown(e: KeyboardEvent): void {
    if ((e.ctrlKey || e.metaKey) && e.key === 'l') { e.preventDefault(); this.enterEditMode(); }
    if (e.altKey && e.key === 'd')                  { e.preventDefault(); this.enterEditMode(); }
    if ((e.ctrlKey || e.metaKey) && e.key === 'q') { e.preventDefault(); this.toggleQuickAccess(); }
    if ((e.ctrlKey || e.metaKey) && e.key === 'b') { e.preventDefault(); this.openQuickAccess('Pinned'); }
  }

  @HostListener('document:keydown', ['$event'])
  onDocKey(e: KeyboardEvent): void {
    if (e.key !== 'Escape') return;
    if (this.ctxMenu.visible)                                                        { this.closeCtx();         e.preventDefault(); }
    else if (this.editMode)                                                          { this.exitEditMode(false); e.preventDefault(); }
    else if (this.chevIdx >= 0 || this.showQuickAccess || this.showSchemeMenu) { this.closeAll();          e.preventDefault(); }
  }
}


