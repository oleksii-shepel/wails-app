import {
  Component,
  Input,
  Output,
  EventEmitter,
  OnInit,
  OnDestroy,
  OnChanges,
  SimpleChanges,
  ViewChild,
  ElementRef,
  HostListener,
  ChangeDetectorRef,
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { Subject, debounceTime, distinctUntilChanged, takeUntil } from 'rxjs';
import { PathHistoryService, PathSuggestion } from '../services/path-history.service';
import { ApiService } from '../services/api.service';
import { DirectoryListing, FileType } from '../shared/protocol';
import { DriveInfo } from '../shared/protocol-enhanced';
import { ArchiveFrame, buildArchiveDisplayPath, isArchive } from '../services/archive.service';

// ─── Types ─────────────────────────────────────────────────────────────────

interface PathSegment {
  label: string;
  fullPath: string;
  isRoot: boolean;
  rootLabel?: string;
  /** True for the archive file entry itself (e.g. project.zip) */
  isArchiveRoot?: boolean;
  /** True for path parts that live *inside* an archive — not navigable via FS */
  isArchiveInner?: boolean;
}

interface SiblingEntry {
  name: string;
  path: string;
  isCurrent: boolean;
}

interface PinnedPath {
  path: string;
  label: string;
  icon: string;
  addedAt: number;
}

interface ContextMenu {
  visible: boolean;
  x: number;
  y: number;
  segmentIndex: number;
  segment: PathSegment | null;
}

const PINNED_KEY = 'filemanager:pinned-paths-v2';

// ─── Utilities ──────────────────────────────────────────────────────────────

function isWindowsPath(p: string): boolean {
  return /^[A-Za-z]:/.test(p);
}

function normalizePath(p: string): string {
  if (!p) return '/';
  let n = p.replace(/\\/g, '/');
  if (isWindowsPath(n)) {
    const drive = n.slice(0, 2);
    const rest = n.slice(2).replace(/\/+/g, '/');
    n = drive + (rest.startsWith('/') ? rest : '/' + rest);
  } else {
    n = n.replace(/\/+/g, '/');
  }
  if (n.length > 1 && n.endsWith('/') && !(/^[A-Za-z]:\//.test(n) && n.length === 3)) {
    n = n.slice(0, -1);
  }
  return n || '/';
}

function buildSegments(p: string): PathSegment[] {
  if (!p) return [];

  if (isWindowsPath(p)) {
    const drive = p.slice(0, 2).toUpperCase();
    const rest = p.slice(3);
    const parts = rest ? rest.split('/').filter(Boolean) : [];

    const segs: PathSegment[] = [{
      label: drive,
      fullPath: drive + '/',
      isRoot: true,
      rootLabel: drive,
    }];

    parts.forEach((part, i) => {
      segs.push({
        label: part,
        fullPath: drive + '/' + parts.slice(0, i + 1).join('/'),
        isRoot: false,
      });
    });
    return segs;
  }

  const parts = p.split('/').filter(Boolean);
  const segs: PathSegment[] = [{
    label: '/',
    fullPath: '/',
    isRoot: true,
    rootLabel: '/',
  }];
  parts.forEach((part, i) => {
    segs.push({
      label: part,
      fullPath: '/' + parts.slice(0, i + 1).join('/'),
      isRoot: false,
    });
  });
  return segs;
}

function parentPath(p: string): string {
  const n = normalizePath(p);
  if (/^[A-Za-z]:\/$/.test(n) || /^[A-Za-z]:$/.test(n)) return n;
  if (n === '/') return '/';
  const parts = n.split('/').filter(Boolean);
  if (parts.length <= 1) {
    return isWindowsPath(n) ? parts[0] + '/' : '/';
  }
  parts.pop();
  if (isWindowsPath(n)) {
    const drive = parts[0];
    const rest = parts.slice(1);
    return rest.length ? drive + '/' + rest.join('/') : drive + '/';
  }
  return '/' + parts.join('/');
}

/**
 * Build breadcrumb segments for an archive stack.
 */
function buildArchiveSegments(fsDirPath: string, stack: ArchiveFrame[]): PathSegment[] {
  if (stack.length === 0) return buildSegments(fsDirPath);

  const virtualPath = buildArchiveDisplayPath(fsDirPath, stack);
  const allSegs = buildSegments(virtualPath);

  const fsSegs = buildSegments(fsDirPath);
  const fsDepth = fsSegs.length;

  allSegs.forEach((seg, i) => {
    if (i === fsDepth - 1) {
      seg.isArchiveRoot = true;
    } else if (i >= fsDepth) {
      const label = seg.label.toLowerCase();
      if (label.endsWith('.zip') || label.endsWith('.tar') || label.endsWith('.tgz') ||
          label.endsWith('.tar.gz') || label.endsWith('.tar.bz2') || label.endsWith('.tar.xz')) {
        seg.isArchiveRoot = true;
        seg.isArchiveInner = true;
      } else {
        seg.isArchiveInner = true;
      }
    }
  });

  return allSegs;
}

// ───────────────────────────────────────────────────────────────────────────

@Component({
  selector: 'app-address-bar',
  standalone: true,
  imports: [CommonModule, FormsModule],
  template: `
<div class="ab-host" (keydown)="onHostKeyDown($event)">

  <!-- ── BREADCRUMB ROW ───────────────────────────────── -->
  <div class="ab-bar" *ngIf="!editMode" [class.ab-bar--compact]="compact">

    <!-- Left cluster: pin-star + pinned-list -->
    <div class="ab-cluster ab-cluster--left">

      <!-- Pin / unpin current path -->
      <button
        class="ab-btn ab-btn--star"
        [class.ab-btn--pinned]="isPinned"
        (click)="togglePin()"
        [title]="isPinned ? 'Unpin this path' : 'Pin this path'"
        aria-label="Toggle pin"
      >{{ isPinned ? '★' : '☆' }}</button>

      <!-- Pinned-paths dropdown trigger -->
      <div class="ab-drop-anchor" #pinAnchor>
        <button
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

        <!-- Pin dropdown -->
        <div class="ab-dropdown ab-pin-drop" *ngIf="showPinDrop">
          <div class="ab-drop-head">
            <span>Pinned Paths</span>
            <span class="ab-badge">{{ pinnedPaths.length }}</span>
          </div>
          <div class="ab-drop-empty" *ngIf="pinnedPaths.length === 0">
            <span class="ab-drop-empty-icon">📍</span>
            <span>No pinned paths yet.<br>Click ☆ to pin the current path.</span>
          </div>
          <div class="ab-drop-scroll">
            <div
              *ngFor="let p of pinnedPaths"
              class="ab-drop-item ab-pin-item"
              [class.ab-drop-item--active]="p.path === currentPath"
              (click)="navigateTo(p.path)"
            >
              <span class="ab-pin-icon">{{ p.icon }}</span>
              <span class="ab-pin-label" [title]="p.path">{{ p.label }}</span>
              <span class="ab-pin-sub">{{ shortenPath(p.path) }}</span>
              <button
                class="ab-pin-remove"
                (click)="removePin(p, $event)"
                title="Remove"
                aria-label="Remove pin"
              >✕</button>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Scrollable breadcrumb trail -->
    <div class="ab-crumbs" (click)="onCrumbsClick($event)">
      <div class="ab-crumbs-inner" #crumbsEl>
        <ng-container *ngFor="let seg of segments; let i = index; let last = last">

          <!-- ARCHIVE-INNER segment (inside an archive, not FS-navigable) -->
          <span
            *ngIf="seg.isArchiveInner && !seg.isArchiveRoot"
            class="ab-seg ab-seg--archive-inner"
            [class.ab-seg--last]="last"
            [class.ab-seg--clickable]="!last"
            [title]="seg.fullPath || seg.label"
            (click)="!last && onArchiveSegClick(seg, $event)"
          >{{ seg.label }}</span>

          <!-- ARCHIVE-ROOT segment (the archive file itself, or a nested archive) -->
          <span
            *ngIf="seg.isArchiveRoot"
            class="ab-seg ab-seg--archive-root"
            [class.ab-seg--clickable]="!last"
            (click)="!last && onArchiveSegClick(seg, $event)"
            [title]="seg.fullPath || seg.label"
          >
            <svg class="ab-seg-archive-icon" width="11" height="11" viewBox="0 0 16 16" fill="currentColor">
              <path d="M6.5 7.5a1 1 0 0 0-1 1v.5h-1v-.5a2 2 0 0 1 4 0v.5h-1v-.5a1 1 0 0 0-1-1zM5.5 11a1 1 0 1 0 2 0v-.5h-2V11zm7-6H10V2.5a.5.5 0 0 0-.5-.5h-3a.5.5 0 0 0-.5.5V5H3.5A1.5 1.5 0 0 0 2 6.5v6A1.5 1.5 0 0 0 3.5 14h9a1.5 1.5 0 0 0 1.5-1.5v-6A1.5 1.5 0 0 0 12.5 5zM7 2h2v3H7V2zm5.5 10.5a.5.5 0 0 1-.5.5h-8a.5.5 0 0 1-.5-.5v-6a.5.5 0 0 1 .5-.5h8a.5.5 0 0 1 .5.5v6z"/>
            </svg>
            {{ seg.label }}
          </span>

          <!-- Archive chevron — now fully interactive, opens sibling dropdown -->
          <div class="ab-chev-wrap" *ngIf="!last && (seg.isArchiveInner || seg.isArchiveRoot)">
            <button
              class="ab-chev ab-chev--archive"
              [class.ab-chev--open]="chevIdx === i"
              (click)="onArchiveChevClick(i, seg, $event)"
              aria-label="Browse archive siblings"
            >›</button>
          </div>

          <!-- DRIVE LETTER (Windows) - VS Code blue, clickable to show drive grid -->
          <span
            *ngIf="seg.isRoot && isWin && !seg.isArchiveInner && !seg.isArchiveRoot"
            class="ab-seg ab-drive-letter"
            [class.ab-seg--last]="last"
            (click)="onDriveLetterClick(seg, $event)"
            (contextmenu)="onSegRightClick(seg, i, $event)"
            [title]="'Switch drives'"
          >
            {{ seg.label }}
          </span>

          <!-- CHEVRON after drive letter (shows root folder contents) -->
          <div class="ab-drive-chevron-wrap" *ngIf="seg.isRoot && isWin && !last && !seg.isArchiveInner && !seg.isArchiveRoot">
            <button
              class="ab-chev ab-drive-chevron"
              [class.ab-chev--open]="chevIdx === i"
              (click)="toggleChev(i, seg, $event)"
              [title]="'Show contents of ' + seg.fullPath"
              aria-label="Show drive contents"
            >
              ›
            </button>
          </div>

          <!-- Regular segment (non-drive or Unix root) -->
          <span
            *ngIf="(!seg.isRoot || !isWin) && !seg.isArchiveInner && !seg.isArchiveRoot"
            class="ab-seg"
            [class.ab-seg--root]="seg.isRoot"
            [class.ab-seg--last]="last"
            (click)="onSegClick(seg, $event)"
            (contextmenu)="onSegRightClick(seg, i, $event)"
            [title]="seg.fullPath"
          >{{ seg.label }}</span>

          <!-- Regular chevron between FS segments -->
          <div class="ab-chev-wrap" *ngIf="!last && !(seg.isRoot && isWin) && !seg.isArchiveInner && !seg.isArchiveRoot">
            <button
              class="ab-chev"
              [class.ab-chev--open]="chevIdx === i"
              (click)="toggleChev(i, seg, $event)"
              aria-label="Browse siblings"
              #chevBtn
            >›</button>
          </div>

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

  <!-- ── EDIT MODE ROW ─────────────────────────────────── -->
  <div class="ab-edit" *ngIf="editMode">
    <span class="ab-edit-icon">📂</span>
    <input
      #editInput
      class="ab-edit-input"
      type="text"
      [(ngModel)]="editValue"
      (ngModelChange)="onEditChange($event)"
      (keydown)="onEditKeyDown($event)"
      (blur)="onEditBlur()"
      [placeholder]="placeholder"
      autocomplete="off"
      spellcheck="false"
    />
    <span class="ab-edit-hint" *ngIf="!editValue">{{ currentPath }}</span>
    <button class="ab-btn ab-btn--cancel" (click)="exitEditMode(false)" title="Cancel (Esc)" aria-label="Cancel">✕</button>
  </div>

  <!-- ── AUTOCOMPLETE DROPDOWN ──────────────────────────── -->
  <div class="ab-dropdown ab-autocomplete" *ngIf="editMode && suggestions.length > 0">
    <div class="ab-drop-head">
      <span>Suggestions</span>
      <span class="ab-badge">{{ suggestions.length }}</span>
    </div>
    <div class="ab-drop-scroll">
      <div
        *ngFor="let s of suggestions; let i = index"
        class="ab-drop-item ab-ac-item"
        [class.ab-drop-item--focused]="acIdx === i"
        (mouseenter)="acIdx = i"
        (mousedown)="acceptSuggestion(s, $event)"
      >
        <span class="ab-ac-badge ab-ac-badge--{{ s.matchType }}">{{ s.matchType }}</span>
        <span class="ab-ac-path" [innerHTML]="highlightMatch(s.path, editValue)"></span>
      </div>
    </div>
    <div class="ab-drop-footer">
      <kbd>↑↓</kbd> navigate &nbsp; <kbd>⏎</kbd> select &nbsp; <kbd>Esc</kbd> cancel
    </div>
  </div>

  <!-- ── DRIVE SWITCHER DROPDOWN (GRID) ─────────────────── -->
  <div class="ab-dropdown ab-drive-drop" *ngIf="showDriveMenu"
    [ngStyle]="{ left: driveMenuLeft + 'px', top: driveMenuTop + 'px', position: 'fixed' }">
    <div class="ab-drop-head">
      <span>Available Drives</span>
      <span class="ab-badge">{{ drives.length }}</span>
    </div>

    <div class="ab-drop-loading" *ngIf="drivesLoading">
      <span class="ab-spinner"></span>Loading drives…
    </div>

    <div class="ab-drop-empty" *ngIf="!drivesLoading && drives.length === 0">
      <span class="ab-drop-empty-icon">💽</span>
      <span>No drives found</span>
    </div>

    <div class="ab-drive-grid" *ngIf="!drivesLoading && drives.length > 0">
      <div
        *ngFor="let d of drives"
        class="ab-drive-grid-item"
        [class.ab-drive-grid-item--active]="d.path === currentPath"
        (click)="switchToDrive(d)"
      >
        <div class="ab-drive-grid-top">
          <span class="ab-drive-grid-icon">{{ getDriveIcon(d) }}</span>
          <span class="ab-drive-grid-letter">{{ d.name }}</span>
          <span class="ab-drive-grid-label">{{ 'Local Drive' }}</span>
          <svg *ngIf="d.path === currentPath" class="ab-drive-grid-check" width="14" height="14" viewBox="0 0 16 16" fill="currentColor">
            <path d="M13.485 1.929L5.5 9.914 2.515 6.929a1 1 0 0 0-1.414 1.414l3.692 3.693a1 1 0 0 0 1.414 0l8.692-8.693a1 1 0 0 0-1.414-1.414z"/>
          </svg>
        </div>

        <div class="ab-drive-grid-details">
          <div class="ab-drive-grid-space" *ngIf="d.totalSpace > 0">
            <div class="ab-drive-grid-space-labels">
              <span class="ab-drive-grid-space-free">{{ formatSize(d.freeSpace) }} free</span>
              <span>of {{ formatSize(d.totalSpace) }}</span>
            </div>
            <div class="ab-drive-space-bar">
              <div class="ab-drive-space-fill"
                   [style.width.%]="(d.freeSpace / d.totalSpace) * 100"></div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div class="ab-drop-footer" *ngIf="!drivesLoading && drives.length > 0">
      <kbd>↑↓</kbd> navigate &nbsp; <kbd>⏎</kbd> select &nbsp; <kbd>Esc</kbd> close
    </div>
  </div>

  <!-- ── SIBLING DROPDOWN — single instance outside ngFor, works for both FS and archive ── -->
  <div
    class="ab-dropdown ab-sib-drop"
    *ngIf="chevIdx >= 0"
    [ngStyle]="{ left: sibDropLeft + 'px', top: sibDropTop + 'px', position: 'fixed' }"
  >
    <div class="ab-drop-head">
      <span>{{ segments[chevIdx].label }}</span>
      <span class="ab-badge" *ngIf="!sibLoading">{{ siblings.length }}</span>
    </div>
    <div class="ab-drop-loading" *ngIf="sibLoading">
      <span class="ab-spinner"></span>Loading…
    </div>
    <div class="ab-drop-empty" *ngIf="!sibLoading && siblings.length === 0">
      <span class="ab-drop-empty-icon">📭</span>
      <span>No sub-folders</span>
    </div>
    <div class="ab-drop-scroll">
      <div
        *ngFor="let s of siblings"
        class="ab-drop-item ab-sib-item"
        [class.ab-drop-item--active]="s.isCurrent"
        (click)="onSibClick(s)"
      >
        <span class="ab-sib-folder">📁</span>
        <span class="ab-sib-name">{{ s.name }}</span>
      </div>
    </div>
  </div>

  <!-- ── RIGHT-CLICK CONTEXT MENU ──────────────────────── -->
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
      /* z-index removed: creates a stacking context that traps fixed children */
    }

    .ab-host {
      position: relative;
      background: var(--vsc-sidebar-background);
      border-bottom: 1px solid var(--vsc-border-subtle);
    }

    /* Main bar */
    .ab-bar {
      display: flex;
      align-items: center;
      height: var(--vsc-titlebar-height);
      padding: 0 var(--vsc-padding-sm);
      gap: var(--vsc-padding-xs);
      overflow: visible;
      background: var(--vsc-sidebar-background);
      transition: background var(--vsc-transition-fast);
    }

    .ab-bar--compact {
      height: 28px;

      .ab-btn {
        width: 22px;
        height: 22px;
      }

      .ab-seg {
        height: 20px;
        line-height: 20px;
        font-size: var(--vsc-font-size-small);
      }
    }

    /* Clusters with VS Code separators */
    .ab-cluster {
      display: flex;
      align-items: center;
      gap: var(--vsc-padding-xs);
      flex-shrink: 0;
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

    /* VS Code style buttons */
    .ab-btn {
      display: flex;
      align-items: center;
      justify-content: center;
      width: 26px;
      height: 26px;
      padding: 0;
      background: transparent;
      border: none;
      border-radius: var(--vsc-border-radius-sm);
      color: var(--vsc-foreground-dim);
      cursor: pointer;
      transition: background-color var(--vsc-transition-fast), color var(--vsc-transition-fast);
      flex-shrink: 0;

      &:hover {
        background: var(--vsc-list-hover-background);
        color: var(--vsc-foreground-bright);
      }

      &:active {
        background: var(--vsc-list-active-background);
        color: var(--vsc-foreground-bright);
      }

      &:focus-visible {
        outline: 1px solid var(--vsc-focusBorder, var(--vsc-accent-blue));
        outline-offset: -1px;
      }

      svg {
        width: 16px;
        height: 16px;
        fill: currentColor;
        opacity: 0.9;
        transition: opacity var(--vsc-transition-fast);
      }

      &:hover svg {
        opacity: 1;
      }
    }

    /* Star/Pin button */
    .ab-btn--star {
      font-size: 16px;
      line-height: 1;

      &:hover {
        color: var(--vsc-accent-yellow);
      }
    }

    .ab-btn--pinned {
      color: var(--vsc-accent-yellow) !important;
    }

    /* Open state for dropdown triggers */
    .ab-btn--open {
      background: var(--vsc-list-active-background);
      color: var(--vsc-foreground-bright);

      svg {
        opacity: 1;
      }
    }

    /* Copy success state */
    .ab-btn--copied {
      color: var(--vsc-success) !important;
    }

    /* Cancel button in edit mode */
    .ab-btn--cancel {
      font-size: 14px;
      color: var(--vsc-foreground-dim);

      &:hover {
        background: var(--vsc-error);
        color: var(--vsc-foreground-bright);
      }
    }

    /* Breadcrumb container */
    .ab-crumbs {
      flex: 1;
      min-width: 0;
      overflow: visible;
      cursor: text;
      height: 100%;
      display: flex;
      align-items: center;
    }

    .ab-crumbs-inner {
      display: flex;
      align-items: center;
      height: 100%;
      min-width: 0;
      overflow-x: auto;
      overflow-y: hidden;
      gap: 0;
      scroll-behavior: smooth;
      scrollbar-width: none;
      -ms-overflow-style: none;

      &::-webkit-scrollbar {
        display: none;
      }
    }

    /* Breadcrumb segments */
    .ab-seg {
      display: inline-flex;
      align-items: center;
      height: 24px;
      padding: 0 var(--vsc-padding-sm);
      border-radius: var(--vsc-border-radius-sm);
      color: var(--vsc-foreground-dim);
      font-size: var(--vsc-font-size);
      line-height: 24px;
      white-space: nowrap;
      cursor: pointer;
      transition: background-color var(--vsc-transition-fast), color var(--vsc-transition-fast);
      max-width: 180px;
      overflow: hidden;
      text-overflow: ellipsis;

      &:hover {
        background: var(--vsc-list-hover-background);
        color: var(--vsc-foreground);
      }

      &.ab-seg--root {
        color: var(--vsc-foreground);
        font-weight: 500;
      }

      &.ab-seg--last {
        color: var(--vsc-foreground-bright);
        cursor: default;
        max-width: 240px;

        &:hover {
          background: transparent;
        }
      }

      /* Archive root segment — the .zip/.tar file itself */
      &.ab-seg--archive-root {
        background: color-mix(in srgb, var(--vsc-accent-orange) 18%, transparent);
        border: 1px solid rgba(255, 140, 0, 0.35);
        color: #ffb347;
        gap: 4px;
        cursor: default;

        &.ab-seg--clickable {
          cursor: pointer;
        }

        &.ab-seg--clickable:hover {
          background: color-mix(in srgb, var(--vsc-accent-orange) 34%, transparent);
          color: #ffb347;
        }

        &:not(.ab-seg--clickable):hover {
          background: color-mix(in srgb, var(--vsc-accent-orange) 18%, transparent);
        }

        .ab-seg-archive-icon {
          opacity: 0.85;
          flex-shrink: 0;
        }
      }

      /* Archive inner segment — a folder inside the archive */
      &.ab-seg--archive-inner {
        color: color-mix(in srgb, #ffb347 80%, var(--vsc-foreground));
        cursor: default;

        &.ab-seg--clickable {
          cursor: pointer;
        }

        &.ab-seg--clickable:hover {
          background: color-mix(in srgb, #ffb347 15%, transparent);
          color: #ffb347;
        }

        &:not(.ab-seg--clickable):hover {
          background: transparent;
        }
      }
    }

    /* Drive letter styling */
    .ab-drive-letter {
      background: color-mix(in srgb, #007fd4 20%, transparent);
      border: 1px solid rgba(0,127,212,0.4);
      border-radius: 4px;
      color: #7ec8f8;
      font-family: var(--vsc-font-family-mono);
      font-weight: 600;
      padding: 0 var(--vsc-padding-lg) !important;
      margin-right: var(--vsc-padding-md) !important;
      cursor: pointer;
      transition: background 0.08s, border-color 0.08s, color 0.08s;

      &:hover {
        background: color-mix(in srgb, #007fd4 30%, transparent);
        border-color: rgba(0,127,212,0.6);
        color: #7ec8f8;
      }

      &.ab-seg--last {
        background: color-mix(in srgb, #007fd4 18%, transparent);
      }
    }

    /* Chevron styling */
    .ab-chev-wrap {
      position: relative;
      flex-shrink: 0;
      overflow: visible;
      display: inline-flex;
      align-items: center;
    }

    /* Base chevron — already has transparent, but be explicit */
    .ab-chev {
      display: flex;
      align-items: center;
      justify-content: center;
      width: 20px;
      height: 24px;
      padding: 0;
      background: transparent !important;
      border: none;
      color: var(--vsc-foreground-dim);
      font-size: 16px;
      font-weight: 400;
      cursor: pointer;
      border-radius: var(--vsc-border-radius-sm);
      transition: color var(--vsc-transition-fast), transform var(--vsc-transition-normal);
      /* REMOVED background-color from transition — prevents colored flash */

      &:hover {
        color: var(--vsc-foreground);
        background: var(--vsc-list-hover-background) !important;
      }

      &:focus-visible {
        outline: 1px solid var(--vsc-accent-blue);
        outline-offset: -1px;
      }

      &.ab-chev--open {
        transform: rotate(90deg);
        color: var(--vsc-foreground-dim);       /* no blue tint when open */
        background: transparent !important;     /* no background when open */
      }

      /* Drive chevron — inherits base, no special background */
      &.ab-drive-chevron {
        border-radius: 0 var(--vsc-border-radius-sm) var(--vsc-border-radius-sm) 0;
        margin-left: -2px;
        background: transparent !important;

        &.ab-chev--open {
          background: transparent !important;
          color: var(--vsc-foreground-dim);
        }
      }

      /* Archive chevron — amber color only, never a background tint at rest */
      &.ab-chev--archive {
        color: rgba(255, 179, 71, 0.45);
        background: transparent !important;

        &:hover {
          background: transparent !important;   /* no hover bg either — matches FS chevrons */
          color: #ffb347;
        }

        &.ab-chev--open {
          transform: rotate(90deg);
          color: #ffb347;
          background: transparent !important;
        }
      }
    }

    /* Drive chevron wrapper */
    .ab-drive-chevron-wrap {
      display: inline-flex;
      align-items: center;
      margin-right: var(--vsc-padding-xs);
    }

    .ab-drive-chevron {
      border-radius: 0 var(--vsc-border-radius-sm) var(--vsc-border-radius-sm) 0;
      margin-left: -2px;

      &.ab-chev--open {
        background: var(--vsc-list-active-background) !important;
        color: var(--vsc-foreground-bright);
      }
    }

    /* === DROPDOWNS === */

    .ab-dropdown {
      position: absolute;
      min-width: 220px;
      max-width: 340px;
      background-color: var(--vsc-panel-background, #252526) !important;
      background-image: none !important;
      border: 1px solid var(--vsc-border);
      border-radius: var(--vsc-border-radius);
      box-shadow: var(--vsc-widget-shadow);
      z-index: 9999;
      overflow: hidden;
      animation: ab-appear var(--vsc-transition-fast) cubic-bezier(0.4, 0, 0.2, 1);
      isolation: isolate;

      .ab-drop-head,
      .ab-drop-scroll,
      .ab-drop-footer,
      .ab-drop-loading,
      .ab-drop-empty {
        background-color: var(--vsc-panel-background, #252526) !important;
      }

      .ab-drop-scroll {
        .ab-drop-item {
          background: transparent;

          &:hover {
            background: var(--vsc-list-hover-background);
          }

          &.ab-drop-item--active {
            background: var(--vsc-list-active-background);
          }
        }
      }
    }

    @keyframes ab-appear {
      from {
        opacity: 0;
        transform: translateY(-4px);
      }
      to {
        opacity: 1;
        transform: translateY(0);
      }
    }

    /* Dropdown header */
    .ab-drop-head {
      display: flex;
      align-items: center;
      justify-content: space-between;
      height: 32px;
      padding: 0 var(--vsc-padding-lg);
      background: var(--vsc-panel-background);
      border-bottom: 1px solid var(--vsc-border-subtle);
      font-size: var(--vsc-font-size-small);
      color: var(--vsc-foreground-dim);
      text-transform: uppercase;
      letter-spacing: 0.5px;
      font-weight: 600;
    }

    /* Badge */
    .ab-badge {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      min-width: 20px;
      height: 20px;
      padding: 0 var(--vsc-padding-xs);
      background: var(--vsc-badge-background);
      color: var(--vsc-badge-foreground);
      border-radius: 10px;
      font-size: 11px;
      font-weight: 600;
      line-height: 1;
    }

    /* Scrollable content */
    .ab-drop-scroll {
      max-height: 300px;
      overflow-y: auto;
      padding: var(--vsc-padding-xs) 0;

      &::-webkit-scrollbar {
        width: 10px;
      }

      &::-webkit-scrollbar-track {
        background: var(--vsc-scrollbar-track);
      }

      &::-webkit-scrollbar-thumb {
        background: var(--vsc-scrollbar-thumb);
        border-radius: 5px;

        &:hover {
          background: var(--vsc-scrollbar-thumb-hover);
        }
      }
    }

    /* Dropdown items */
    .ab-drop-item {
      display: flex;
      align-items: center;
      gap: var(--vsc-padding-sm);
      height: 28px;
      padding: 0 var(--vsc-padding-lg);
      cursor: pointer;
      transition: background-color var(--vsc-transition-fast);
      border-left: 2px solid transparent;
      color: var(--vsc-foreground);
      font-size: var(--vsc-font-size);

      &:hover {
        background: var(--vsc-list-hover-background);
      }

      &.ab-drop-item--active {
        background: var(--vsc-list-active-background);
        color: var(--vsc-foreground-bright);
        border-left-color: var(--vsc-accent-blue);

        &:hover {
          background: var(--vsc-list-active-background);
        }

        .ab-pin-sub {
          color: var(--vsc-foreground-bright);
          opacity: 0.9;
        }
      }

      &.ab-drop-item--focused {
        background: var(--vsc-list-focus-background);
      }
    }

    /* Checkbox */
    .ab-checkbox {
      flex-shrink: 0;
      display: flex;
      align-items: center;
      justify-content: center;
      width: 16px;
      height: 16px;
      border: 1px solid var(--vsc-border);
      border-radius: var(--vsc-border-radius-sm);
      background: var(--vsc-input-background);
      transition: all var(--vsc-transition-fast);
      color: transparent;

      &.ab-checkbox--checked {
        background: var(--vsc-accent-blue) !important;
        border-color: var(--vsc-accent-blue) !important;
        color: var(--vsc-foreground-bright);

        &::after {
          content: '✓';
          font-size: 12px;
          line-height: 1;
        }
      }
    }

    /* Dropdown footer */
    .ab-drop-footer {
      display: flex;
      align-items: center;
      gap: var(--vsc-padding-sm);
      height: 28px;
      padding: 0 var(--vsc-padding-lg);
      background: var(--vsc-panel-background);
      border-top: 1px solid var(--vsc-border-subtle);
      font-size: 11px;
      color: var(--vsc-foreground-dim);

      kbd {
        display: inline-flex;
        align-items: center;
        justify-content: center;
        min-width: 18px;
        height: 18px;
        padding: 0 3px;
        background: var(--vsc-input-background);
        border: 1px solid var(--vsc-border);
        border-bottom-width: 2px;
        border-radius: var(--vsc-border-radius-sm);
        font-family: var(--vsc-font-family-mono);
        font-size: 10px;
        color: var(--vsc-foreground);
        line-height: 1;
        margin: 0 var(--vsc-padding-xs);
      }
    }

    /* Loading state */
    .ab-drop-loading {
      display: flex;
      align-items: center;
      gap: var(--vsc-padding-sm);
      padding: var(--vsc-padding-lg);
      font-size: var(--vsc-font-size);
      color: var(--vsc-foreground-dim);
    }

    .ab-spinner {
      display: inline-block;
      width: 14px;
      height: 14px;
      border: 2px solid var(--vsc-border);
      border-top-color: var(--vsc-accent-blue);
      border-radius: 50%;
      animation: ab-spin 0.7s linear infinite;
    }

    @keyframes ab-spin {
      to { transform: rotate(360deg); }
    }

    /* Empty state */
    .ab-drop-empty {
      display: flex;
      flex-direction: column;
      align-items: center;
      gap: var(--vsc-padding-sm);
      padding: var(--vsc-padding-xl);
      font-size: var(--vsc-font-size-small);
      color: var(--vsc-foreground-dim);
      text-align: center;
      line-height: 1.5;
    }

    .ab-drop-empty-icon {
      font-size: 24px;
      opacity: 0.5;
      margin-bottom: var(--vsc-padding-xs);
    }

    /* === PIN DROPDOWN === */
    .ab-pin-drop {
      min-width: 270px;
      max-width: 370px;
    }

    .ab-pin-item {
      padding-right: var(--vsc-padding-sm);
    }

    .ab-pin-icon {
      font-size: 14px;
      flex-shrink: 0;
      opacity: 0.8;
    }

    .ab-pin-label {
      flex: 1;
      font-size: var(--vsc-font-size);
      font-weight: 500;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }

    .ab-pin-sub {
      font-size: 11px;
      color: var(--vsc-foreground-dim);
      font-family: var(--vsc-font-family-mono);
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
      max-width: 120px;
    }

    .ab-pin-remove {
      flex-shrink: 0;
      width: 20px;
      height: 20px;
      padding: 0;
      background: transparent;
      border: none;
      color: transparent;
      font-size: 12px;
      cursor: pointer;
      border-radius: var(--vsc-border-radius-sm);
      transition: all var(--vsc-transition-fast);
      display: flex;
      align-items: center;
      justify-content: center;

      .ab-drop-item:hover & {
        color: var(--vsc-foreground-dim);
      }

      &:hover {
        background: var(--vsc-error) !important;
        color: var(--vsc-foreground-bright) !important;
      }
    }

    /* === SIBLING DROPDOWN === */
    .ab-sib-drop {
      min-width: 210px;
      max-width: 320px;
    }

    .ab-sib-folder {
      font-size: 14px;
      flex-shrink: 0;
      opacity: 0.8;
    }

    .ab-sib-name {
      flex: 1;
      font-size: var(--vsc-font-size);
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }

    /* === DRIVE DROPDOWN GRID === */
    .ab-drive-drop {
      min-width: 320px;
      max-width: 500px;
      padding: 0;
      z-index: 9999 !important;
      position: fixed !important;
      background-color: var(--vsc-panel-background, #252526) !important;
      background-image: none !important;
      border: 1px solid var(--vsc-border);
      border-radius: var(--vsc-border-radius);
      box-shadow: var(--vsc-widget-shadow);
      isolation: isolate;

      .ab-drop-head {
        background-color: var(--vsc-panel-background, #252526) !important;
        border-bottom: 1px solid var(--vsc-border-subtle);
      }

      .ab-drive-grid {
        background-color: var(--vsc-panel-background, #252526) !important;

        .ab-drive-grid-item {
          background-color: var(--vsc-sidebar-background, #1e1e1e) !important;

          &:hover {
            background-color: var(--vsc-list-hover-background) !important;
          }

          &.ab-drive-grid-item--active {
            background-color: var(--vsc-list-active-background) !important;
          }
        }
      }

      .ab-drop-footer {
        background-color: var(--vsc-panel-background, #252526) !important;
        border-top: 1px solid var(--vsc-border-subtle);
      }
    }

    .ab-drive-grid {
      display: grid;
      grid-template-columns: repeat(auto-fill, minmax(160px, 1fr));
      gap: var(--vsc-padding-sm);
      padding: var(--vsc-padding-lg);
      max-height: 350px;
      overflow-y: auto;

      &::-webkit-scrollbar {
        width: 10px;
      }

      &::-webkit-scrollbar-track {
        background: var(--vsc-scrollbar-track);
      }

      &::-webkit-scrollbar-thumb {
        background: var(--vsc-scrollbar-thumb);
        border-radius: 5px;
      }
    }

    .ab-drive-grid-item {
      display: flex;
      flex-direction: column;
      padding: var(--vsc-padding-lg);
      background: var(--vsc-sidebar-background);
      border: 1px solid var(--vsc-border-subtle);
      border-radius: var(--vsc-border-radius);
      cursor: pointer;
      transition: all var(--vsc-transition-fast);
      gap: var(--vsc-padding-sm);

      &:hover {
        background: var(--vsc-list-hover-background);
        border-color: var(--vsc-accent-blue);
        transform: translateY(-1px);
        box-shadow: 0 2px 8px var(--vsc-shadow);
      }

      &.ab-drive-grid-item--active {
        background: var(--vsc-list-active-background);
        border-color: var(--vsc-accent-blue);

        .ab-drive-grid-letter,
        .ab-drive-grid-label,
        .ab-drive-grid-details {
          color: var(--vsc-foreground-bright);
        }
      }
    }

    .ab-drive-grid-top {
      display: flex;
      align-items: center;
      gap: var(--vsc-padding-sm);
    }

    .ab-drive-grid-icon {
      font-size: 20px;
      flex-shrink: 0;
    }

    .ab-drive-grid-letter {
      font-family: var(--vsc-font-family-mono);
      font-size: 14px;
      font-weight: 700;
      color: var(--vsc-accent-blue);
    }

    .ab-drive-grid-label {
      font-size: var(--vsc-font-size);
      font-weight: 500;
      color: var(--vsc-foreground);
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
      flex: 1;
    }

    .ab-drive-grid-check {
      color: var(--vsc-accent-blue);
      margin-left: auto;
      flex-shrink: 0;
      width: 14px;
      height: 14px;
    }

    .ab-drive-grid-details {
      display: flex;
      flex-direction: column;
      gap: 5px;
      font-size: 11px;
      color: var(--vsc-foreground-dim);
      margin-top: 2px;
    }

    .ab-drive-grid-path {
      font-family: var(--vsc-font-family-mono);
      font-size: 10px;
      color: var(--vsc-foreground-dim);
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }

    .ab-drive-grid-space {
      display: flex;
      flex-direction: column;
      gap: 4px;
    }

    .ab-drive-grid-space-labels {
      display: flex;
      align-items: center;
      justify-content: space-between;
      font-size: 10px;
      color: var(--vsc-foreground-dim);
    }

    .ab-drive-grid-space-free {
      font-weight: 600;
      color: var(--vsc-foreground);
    }

    .ab-drive-space-bar {
      width: 100%;
      height: 4px;
      background: var(--vsc-border-subtle);
      border-radius: 2px;
      overflow: hidden;
    }

    .ab-drive-space-fill {
      height: 100%;
      background: var(--vsc-accent-blue);
      border-radius: 2px;
      transition: width var(--vsc-transition-normal);
    }

    /* === EDIT MODE === */
    .ab-edit {
      display: flex;
      align-items: center;
      gap: var(--vsc-padding-sm);
      height: var(--vsc-titlebar-height);
      padding: 0 var(--vsc-padding-md);
      background: var(--vsc-input-background);
      border: 1px solid var(--vsc-accent-blue);
      border-radius: var(--vsc-border-radius);
      animation: ab-edit-in var(--vsc-transition-fast) ease;
      position: relative;
    }

    @keyframes ab-edit-in {
      from {
        background: color-mix(in srgb, var(--vsc-accent-blue) 10%, var(--vsc-input-background));
        border-color: color-mix(in srgb, var(--vsc-accent-blue) 50%, transparent);
      }
      to {
        background: var(--vsc-input-background);
        border-color: var(--vsc-accent-blue);
      }
    }

    .ab-edit-icon {
      font-size: 14px;
      flex-shrink: 0;
      opacity: 0.8;
      color: var(--vsc-foreground-dim);
    }

    .ab-edit-input {
      flex: 1;
      background: transparent;
      border: none;
      outline: none;
      color: var(--vsc-input-foreground);
      font-family: var(--vsc-font-family-mono);
      font-size: var(--vsc-font-size);
      caret-color: var(--vsc-accent-blue);
      min-width: 0;
      padding: 0;

      &::placeholder {
        color: var(--vsc-input-placeholder);
        opacity: 1;
      }

      &::selection {
        background: color-mix(in srgb, var(--vsc-accent-blue) 40%, transparent);
      }
    }

    .ab-edit-hint {
      position: absolute;
      left: calc(32px + var(--vsc-padding-sm));
      font-family: var(--vsc-font-family-mono);
      font-size: var(--vsc-font-size);
      color: var(--vsc-input-placeholder);
      pointer-events: none;
      white-space: nowrap;
      overflow: hidden;
      opacity: 0.7;
    }

    /* === AUTOCOMPLETE === */
    .ab-autocomplete {
      max-height: 350px;
      overflow-y: auto;
    }

    .ab-ac-badge {
      flex-shrink: 0;
      font-size: 10px;
      font-weight: 700;
      text-transform: uppercase;
      letter-spacing: 0.4px;
      padding: 2px var(--vsc-padding-xs);
      border-radius: var(--vsc-border-radius-sm);
      min-width: 50px;
      text-align: center;

      &.ab-ac-badge--history {
        background: var(--vsc-success);
        color: var(--vsc-foreground-bright);
      }

      &.ab-ac-badge--directory {
        background: var(--vsc-accent-blue);
        color: var(--vsc-foreground-bright);
      }

      &.ab-ac-badge--fuzzy {
        background: var(--vsc-accent-orange);
        color: var(--vsc-foreground-bright);
      }
    }

    .ab-ac-path {
      flex: 1;
      font-size: var(--vsc-font-size);
      font-family: var(--vsc-font-family-mono);
      color: var(--vsc-foreground);
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;

      ::ng-deep mark {
        background: var(--vsc-accent-blue);
        color: var(--vsc-foreground-bright);
        padding: 0 2px;
        border-radius: 2px;
        font-style: normal;
        font-weight: 500;
      }
    }

    /* === CONTEXT MENU === */
    .ab-ctx-menu {
      position: fixed;
      min-width: 200px;
      background: var(--vsc-panel-background);
      border: 1px solid var(--vsc-border);
      border-radius: var(--vsc-border-radius);
      box-shadow: var(--vsc-widget-shadow);
      z-index: calc(var(--vsc-z-modal) + 100);
      padding: var(--vsc-padding-xs) 0;
      animation: ab-appear var(--vsc-transition-fast) cubic-bezier(0.2, 0, 0.2, 1);
    }

    .ab-ctx-path {
      padding: var(--vsc-padding-sm) var(--vsc-padding-lg);
      font-size: 11px;
      font-family: var(--vsc-font-family-mono);
      color: var(--vsc-foreground-dim);
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
      max-width: 240px;
      border-bottom: 1px solid var(--vsc-border-subtle);
      margin-bottom: var(--vsc-padding-xs);
    }

    .ab-ctx-divider {
      margin: var(--vsc-padding-xs) 0;
      border: none;
      border-top: 1px solid var(--vsc-border-subtle);
    }

    .ab-ctx-item {
      display: flex;
      align-items: center;
      gap: var(--vsc-padding-sm);
      width: 100%;
      padding: var(--vsc-padding-sm) var(--vsc-padding-lg);
      background: transparent;
      border: none;
      color: var(--vsc-foreground);
      font-size: var(--vsc-font-size);
      font-family: var(--vsc-font-family);
      cursor: pointer;
      text-align: left;
      transition: background-color var(--vsc-transition-fast);

      svg {
        width: 14px;
        height: 14px;
        fill: currentColor;
        opacity: 0.8;
        flex-shrink: 0;
      }

      &:hover {
        background: var(--vsc-list-hover-background);

        svg {
          opacity: 1;
        }
      }

      &:active {
        background: var(--vsc-list-active-background);
        color: var(--vsc-foreground-bright);
      }
    }

    /* === DROP ANCHOR === */
    .ab-drop-anchor {
      position: relative;
    }

    /* === COMPACT MODE OVERRIDES === */
    :host-context(.compact-mode) {
      .ab-bar {
        height: 28px;
      }

      .ab-edit {
        height: 28px;
      }

      .ab-btn {
        width: 22px;
        height: 22px;

        svg {
          width: 14px;
          height: 14px;
        }
      }

      .ab-chev {
        width: 18px;
        height: 22px;
      }

      .ab-drive-letter {
        font-size: var(--vsc-font-size-small);
        padding: 0 var(--vsc-padding-md);
        height: 22px;
        line-height: 22px;
      }

      .ab-seg {
        height: 22px;
        line-height: 22px;
        font-size: var(--vsc-font-size-small);
        padding: 0 var(--vsc-padding-sm);
      }

      .ab-drive-grid-item {
        padding: var(--vsc-padding-sm);
      }

      .ab-drop-item {
        height: 26px;
      }
    }

    /* === LIGHT THEME ADJUSTMENTS === */
    :host-context(.vscode-light) {
      .ab-drive-letter {
        background: color-mix(in srgb, #0060c0 15%, transparent);
        border-color: rgba(0,96,192,0.4);
        color: #0060c0;
      }

      .ab-btn--star:hover {
        color: var(--vsc-accent-yellow);
      }

      .ab-checkbox--checked {
        background: var(--vsc-accent-blue) !important;
      }

      .ab-pin-remove:hover {
        background: var(--vsc-error) !important;
      }
    }
  `],
})
export class AddressBarComponent implements OnInit, OnDestroy, OnChanges {
  @ViewChild('editInput') editInputRef?: ElementRef<HTMLInputElement>;
  @ViewChild('crumbsEl') crumbsEl?: ElementRef<HTMLDivElement>;

  @Input() initialPath = '/';
  @Input() placeholder = 'Enter path…';
  @Input() paneId = 'left';
  @Input() archiveStack: ArchiveFrame[] = [];
  @Input() fsDirPath = '';

  @Output() pathChange = new EventEmitter<string>();
  @Output() navigateUpClicked = new EventEmitter<void>();
  @Output() refreshClicked = new EventEmitter<void>();
  @Output() openInOtherPane = new EventEmitter<string>();
  @Output() archiveSegmentClick = new EventEmitter<string>();

  // ── State: path ─────────────────────────────────────────
  currentPath = '/';
  segments: PathSegment[] = [];

  get isWin(): boolean {
    return isWindowsPath(this.currentPath);
  }

  // ── State: drive switcher ────────────────────────────────
  drives: DriveInfo[] = [];
  showDriveMenu = false;
  driveMenuLeft = 0;
  driveMenuTop = 0;
  drivesLoading = false;

  // ── State: chevron / siblings ────────────────────────────
  chevIdx = -1;
  siblings: SiblingEntry[] = [];
  sibLoading = false;
  sibDropLeft = 0;
  sibDropTop = 0;

  // ── State: pinned paths ──────────────────────────────────
  pinnedPaths: PinnedPath[] = [];
  showPinDrop = false;
  get isPinned(): boolean { return this.pinnedPaths.some(p => p.path === this.currentPath); }

  // ── State: edit mode ─────────────────────────────────────
  editMode = false;
  editValue = '';

  // ── State: autocomplete ──────────────────────────────────
  suggestions: PathSuggestion[] = [];
  acIdx = -1;

  // ── State: context menu ──────────────────────────────────
  ctxMenu: ContextMenu = { visible: false, x: 0, y: 0, segmentIndex: -1, segment: null };

  // ── State: copy feedback ─────────────────────────────────
  justCopied = false;

  // ── Compact passthrough ──────────────────────────────────
  compact = false;

  private destroy$ = new Subject<void>();
  private editChange$ = new Subject<string>();
  private blurTimer: any;
  private copiedTimer: any;

  constructor(
    private pathHistoryService: PathHistoryService,
    private apiService: ApiService,
    private elRef: ElementRef,
    private cdr: ChangeDetectorRef,
  ) {}

  // ────────────────────────────────────────────────────────
  // Lifecycle
  // ────────────────────────────────────────────────────────

  ngOnInit(): void {
    this.currentPath = normalizePath(this.initialPath);
    this.rebuildSegments();
    this.loadPins();

    this.editChange$
      .pipe(debounceTime(240), distinctUntilChanged(), takeUntil(this.destroy$))
      .subscribe(v => this.fetchSuggestions(v));

    if (this.isWin) {
      this.loadDrives();
    }
  }

  ngOnChanges(changes: SimpleChanges): void {
    if (changes['initialPath'] && !changes['initialPath'].firstChange) {
      const next = normalizePath(changes['initialPath'].currentValue);
      if (next !== this.currentPath) {
        this.currentPath = next;
        this.rebuildSegments();
        if (this.editMode) this.editValue = next;
      }
    }
    if (changes['archiveStack'] || changes['fsDirPath']) {
      this.rebuildSegments();
    }
  }

  ngOnDestroy(): void {
    clearTimeout(this.blurTimer);
    clearTimeout(this.copiedTimer);
    this.destroy$.next();
    this.destroy$.complete();
  }

  // ────────────────────────────────────────────────────────
  // Drive handling
  // ────────────────────────────────────────────────────────

  onDriveLetterClick(seg: PathSegment, e: MouseEvent): void {
    e.stopPropagation();
    if (this.showDriveMenu) {
      this.showDriveMenu = false;
      return;
    }
    this.closeAll();
    const rect = (e.target as HTMLElement).getBoundingClientRect();
    this.driveMenuLeft = rect.left;
    this.driveMenuTop = rect.bottom + 4;
    this.showDriveMenu = true;
    if (this.drives.length === 0 && !this.drivesLoading) {
      this.loadDrives();
    }
  }

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
        if (parts.length >= 2) {
          target = '//' + parts[0] + '/' + parts[1];
        } else {
          target = n;
        }
      }
    }

    if (target && target !== this.currentPath) {
      this.navigateTo(target);
    }
  }

  async loadDrives(): Promise<void> {
    if (!this.isWin) return;
    this.drivesLoading = true;
    try {
      const drives = await this.apiService.listDrives();
      this.drives = drives || [];
    } catch (error) {
      console.error('Failed to load drives', error);
      this.drives = [];
    } finally {
      this.drivesLoading = false;
      this.cdr.detectChanges();
    }
  }

  getDriveIcon(drive: DriveInfo): string {
    if (drive.driveType === 'fixed') return '💽';
    if (drive.driveType === 'removable') return '💾';
    if (drive.driveType === 'cdrom') return '💿';
    if (drive.driveType === 'network') return '🌐';
    if (drive.driveType === 'ramdisk') return '⚡';
    return '💽';
  }

  formatSize(bytes: number): string {
    if (bytes === 0) return '0 B';
    const units = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(1024));
    return (bytes / Math.pow(1024, i)).toFixed(1) + ' ' + units[i];
  }

  // ────────────────────────────────────────────────────────
  // Segments
  // ────────────────────────────────────────────────────────

  private rebuildSegments(): void {
    if (this.archiveStack.length > 0 && this.fsDirPath) {
      this.segments = buildArchiveSegments(this.fsDirPath, this.archiveStack);
    } else {
      this.segments = buildSegments(this.currentPath);
    }
    setTimeout(() => {
      if (this.crumbsEl) {
        const el = this.crumbsEl.nativeElement;
        el.scrollLeft = el.scrollWidth;
      }
    }, 0);
  }

  onSegClick(seg: PathSegment, e: MouseEvent): void {
    e.stopPropagation();
    this.closeAll();
    if (seg.isArchiveInner || seg.isArchiveRoot) return;
    if (seg.fullPath !== this.currentPath) {
      this.navigateTo(seg.fullPath);
    }
  }

  // ────────────────────────────────────────────────────────
  // Chevron / sibling dropdown
  // ────────────────────────────────────────────────────────

  /**
   * Called by archive chevrons. Delegates to toggleChev so the sibling
   * dropdown opens — does NOT navigate like the old onArchiveChevClick did.
   */
  onArchiveChevClick(idx: number, seg: PathSegment, e: MouseEvent): void {
    this.toggleChev(idx, seg, e);
  }

  async toggleChev(idx: number, seg: PathSegment, e: MouseEvent): Promise<void> {
    e.stopPropagation();
    this.showPinDrop = false;
    this.showDriveMenu = false;

    // Toggle off if already open at this index
    if (this.chevIdx === idx) {
      this.chevIdx = -1;
      this.siblings = [];
      return;
    }

    const target = e.target as HTMLElement;
    const rect = target.getBoundingClientRect();
    this.sibDropLeft = Math.round(rect.left);
    this.sibDropTop = Math.round(rect.bottom + 4);

    this.chevIdx = idx;
    this.siblings = [];
    this.sibLoading = true;

    const activeChildPath = idx + 1 < this.segments.length
      ? this.segments[idx + 1].fullPath
      : null;

    try {
      // ── Archive segment ───────────────────────────────────────────
      if (seg.isArchiveRoot || seg.isArchiveInner) {
        await this.loadArchiveSiblings(seg, activeChildPath);
        return;
      }

      // ── Normal FS segment ─────────────────────────────────────────
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
    }
  }

  /**
   * Load the sibling directories for an archive segment.
   *
   * "Siblings of seg" = all directories inside seg's *parent* folder
   * within the same archive level.
   *
   * seg.fullPath is a virtual path like "C:/Desktop.zip/src/utils".
   * We strip the fsDirPath prefix to get the archive-internal portion,
   * then go one level up to find the parent innerPath, then call
   * ArchiveBrowse on that parent to list its children.
   */
private async loadArchiveSiblings(
  seg: PathSegment,
  activeChildPath: string | null,
): Promise<void> {
  const stack = this.archiveStack;

  console.group('[loadArchiveSiblings]');
  console.log('seg:', { label: seg.label, fullPath: seg.fullPath, isArchiveRoot: seg.isArchiveRoot, isArchiveInner: seg.isArchiveInner });
  console.log('activeChildPath:', activeChildPath);
  console.log('fsDirPath:', this.fsDirPath);
  console.log('archiveStack:', JSON.stringify(stack.map(f => ({
    fsPath: f.fsPath,
    nestedChain: f.nestedChain,
    innerPath: f.innerPath,
  })), null, 2));

  if (!stack.length) {
    console.warn('→ stack is empty, aborting');
    console.groupEnd();
    this.sibLoading = false;
    return;
  }

  const fsPath       = stack[0].fsPath.replace(/\\/g, '/');
  const vNorm        = seg.fullPath.replace(/\\/g, '/');
  const afterArchive = vNorm.startsWith(fsPath)
    ? vNorm.slice(fsPath.length).replace(/^\//, '')
    : '';
  const allParts     = afterArchive ? afterArchive.split('/').filter(Boolean) : [];

  console.log('--- path decomposition ---');
  console.log('fsPath (stack[0].fsPath):', fsPath);
  console.log('vNorm (seg.fullPath):', vNorm);
  console.log('afterArchive:', afterArchive || '(empty = seg IS the root archive)');
  console.log('allParts:', allParts);

  // Find the deepest frame whose nestedChain is a prefix of allParts
  let owningFrame = stack[0];
  for (const f of stack) {
    if (f.nestedChain.length > allParts.length) continue;
    const prefix = allParts.slice(0, f.nestedChain.length).join('/');
    if (f.nestedChain.join('/') === prefix) {
      owningFrame = f;
    }
  }

  const innerParts = allParts.slice(owningFrame.nestedChain.length);

  console.log('--- frame resolution ---');
  console.log('owningFrame:', { fsPath: owningFrame.fsPath, nestedChain: owningFrame.nestedChain, innerPath: owningFrame.innerPath });
  console.log('innerParts:', innerParts);

  // Determine browse target:
  // - archive-root seg (e.g. Desktop.zip itself) → browse parent context to show siblings
  // - archive-inner seg (a folder inside) → browse parent folder within owningFrame
  let browseChain: string[];
  let browseInnerPath: string;

  if (seg.isArchiveRoot && innerParts.length === 0) {
    // Is there a frame that IS this archive (i.e. we've navigated inside it)?
    // If yes → show its contents. If no → show siblings at parent level.
    const frameInsideThisArchive = stack.find(f =>
      f.nestedChain.length > 0 &&
      f.nestedChain[f.nestedChain.length - 1] === seg.label
    );

    if (frameInsideThisArchive) {
      // We're inside this archive — browse its root to show contents
      browseChain     = frameInsideThisArchive.nestedChain;
      browseInnerPath = '';
      console.log('--- mode: ARCHIVE-ROOT (navigated inside) → browsing its contents ---');
    } else {
      // We're not inside this archive — show siblings at parent level
      browseChain     = owningFrame.nestedChain.slice(0, -1);
      browseInnerPath = '';
      console.log('--- mode: ARCHIVE-ROOT (not entered) → browsing parent context ---');
    }
  } else {
    browseChain     = owningFrame.nestedChain;
    browseInnerPath = innerParts.join('/');
    console.log('--- mode: INNER folder → browsing parent folder within owningFrame ---');
  }

  console.log('ArchiveBrowse call args:', { fsPath, browseChain, browseInnerPath });

  try {
    const app = (window as any).go?.main?.App;
    if (!app?.ArchiveBrowse) throw new Error('ArchiveBrowse not available on window.go.main.App');

    const listing = await app.ArchiveBrowse(fsPath, browseChain, browseInnerPath);

    console.log('ArchiveBrowse raw response:', listing);
    console.log('entries count:', listing?.entries?.length ?? 'NO ENTRIES FIELD');
    console.log('all entry types:', listing?.entries?.map((e: any) => ({ name: e.name, type: e.type })));

    const activeChildVirtual = activeChildPath?.replace(/\\/g, '/') ?? null;

    const filtered = (listing.entries as any[]).filter(e =>
      e.type === 'directory'  || e.type === 'DIRECTORY' ||
      e.type === 'ARCHIVE'    ||
      e.type === 2            ||
      ((e.type === 'file' || e.type === 'FILE' || e.type === 1) &&
        /\.(zip|tar|tgz|gz|bz2|xz)$/i.test(e.name))
    );
    console.log('filtered (dirs + archives):', filtered.map((e: any) => ({ name: e.name, type: e.type })));

    this.siblings = filtered
      .map(e => {
        const sibInnerPath   = browseInnerPath ? `${browseInnerPath}/${e.name}` : e.name;
        const chainPart      = browseChain.length ? `/${browseChain.join('/')}` : '';
        const virtualSibPath = `${fsPath}${chainPart}/${sibInnerPath}`.replace(/\/+/g, '/');
        console.log(`  sibling: ${e.name} (${e.type}) → ${virtualSibPath}`);
        return {
          name:      e.name,
          path:      virtualSibPath,
          isArchive: e.type === 'ARCHIVE' || /\.(zip|tar|tgz|gz|bz2|xz)$/i.test(e.name),
          isCurrent: activeChildVirtual
            ? activeChildVirtual === virtualSibPath ||
              activeChildVirtual.startsWith(virtualSibPath + '/') ||  // ← add this
              activeChildVirtual.endsWith('/' + e.name)
            : false,
        } as SiblingEntry;
      })
      .sort((a, b) => a.name.localeCompare(b.name));

    console.log('final siblings:', this.siblings);

  } catch (err) {
    console.error('ArchiveBrowse FAILED:', err);
    this.siblings = [];
  } finally {
    this.sibLoading = false;
    console.groupEnd();
    this.cdr.detectChanges();
  }
}

  onSibClick(s: SiblingEntry): void {
    this.chevIdx = -1;
    this.siblings = [];
    // Archive siblings carry a virtual path — emit to pane for navigation.
    if (this.archiveStack.length > 0) {
      this.archiveSegmentClick.emit(s.path);
    } else {
      this.navigateTo(s.path);
    }
  }

  onCrumbsClick(e: MouseEvent): void {
    const target = e.target as HTMLElement;
    if (target.classList.contains('ab-crumbs')) {
      this.enterEditMode();
    }
  }

  // ────────────────────────────────────────────────────────
  // Pinned paths
  // ────────────────────────────────────────────────────────

  private loadPins(): void {
    try {
      const raw = localStorage.getItem(PINNED_KEY);
      this.pinnedPaths = raw ? JSON.parse(raw) : [];
    } catch { this.pinnedPaths = []; }
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

  shortenPath(p: string): string {
    if (p.length <= 28) return p;
    const parts = p.split('/').filter(Boolean);
    if (parts.length <= 2) return p;
    return '…/' + parts.slice(-2).join('/');
  }

  // ────────────────────────────────────────────────────────
  // Edit mode
  // ────────────────────────────────────────────────────────

  enterEditMode(): void {
    if (this.archiveStack.length > 0) return;
    this.closeAll();
    this.editMode = true;
    this.editValue = this.currentPath;
    this.suggestions = [];
    this.acIdx = -1;
    setTimeout(() => {
      const el = this.editInputRef?.nativeElement;
      if (el) { el.focus(); el.select(); }
    }, 30);
  }

  exitEditMode(commit: boolean): void {
    clearTimeout(this.blurTimer);
    const val = normalizePath(this.editValue);
    this.editMode = false;
    this.editValue = '';
    this.suggestions = [];
    this.acIdx = -1;
    if (commit && val && val !== this.currentPath) {
      this.navigateTo(val);
    }
  }

  onEditBlur(): void {
    this.blurTimer = setTimeout(() => this.exitEditMode(false), 170);
  }

  onEditChange(v: string): void {
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
        if (this.suggestions.length) this.acceptSuggestion(this.suggestions[0]);
        break;
    }
  }

  onHostKeyDown(e: KeyboardEvent): void {
    if ((e.ctrlKey || e.metaKey) && e.key === 'l') {
      e.preventDefault();
      this.enterEditMode();
    }
  }

  // ────────────────────────────────────────────────────────
  // Autocomplete
  // ────────────────────────────────────────────────────────

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
    const p = normalizePath(s.path);
    this.editMode = false;
    this.editValue = '';
    this.suggestions = [];
    this.navigateTo(p);
  }

  highlightMatch(text: string, query: string): string {
    if (!query) return text;
    const idx = text.toLowerCase().indexOf(query.toLowerCase());
    if (idx < 0) return text;
    return text.slice(0, idx)
      + '<mark>' + text.slice(idx, idx + query.length) + '</mark>'
      + text.slice(idx + query.length);
  }

  // ────────────────────────────────────────────────────────
  // Context menu
  // ────────────────────────────────────────────────────────

  onSegRightClick(seg: PathSegment, idx: number, e: MouseEvent): void {
    e.preventDefault();
    e.stopPropagation();
    this.closeAll();
    this.ctxMenu = { visible: true, x: e.clientX, y: e.clientY, segmentIndex: idx, segment: seg };
  }

  ctxNavigateTo(): void {
    if (this.ctxMenu.segment) this.navigateTo(this.ctxMenu.segment.fullPath);
    this.closeCtx();
  }

  async ctxCopyPath(): Promise<void> {
    if (this.ctxMenu.segment) {
      await navigator.clipboard.writeText(this.ctxMenu.segment.fullPath).catch(() => {});
    }
    this.closeCtx();
  }

  async ctxCopyName(): Promise<void> {
    if (this.ctxMenu.segment) {
      await navigator.clipboard.writeText(this.ctxMenu.segment.label).catch(() => {});
    }
    this.closeCtx();
  }

  ctxOpenOtherPane(): void {
    if (this.ctxMenu.segment) this.openInOtherPane.emit(this.ctxMenu.segment.fullPath);
    this.closeCtx();
  }

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

  private closeCtx(): void {
    this.ctxMenu = { visible: false, x: 0, y: 0, segmentIndex: -1, segment: null };
  }

  // ────────────────────────────────────────────────────────
  // Action buttons
  // ────────────────────────────────────────────────────────

  goUp(): void {
    this.navigateUpClicked.emit();
    if (this.archiveStack.length === 0) {
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

  onArchiveSegClick(seg: PathSegment, e: MouseEvent): void {
    e.stopPropagation();
    this.closeAll();
    this.archiveSegmentClick.emit(seg.fullPath);
  }

  // ────────────────────────────────────────────────────────
  // Navigation
  // ────────────────────────────────────────────────────────

  navigateTo(p: string): void {
    const np = normalizePath(p);
    this.closeAll();
    if (np !== this.currentPath) {
      this.currentPath = np;
      this.rebuildSegments();
      this.pathChange.emit(np);
    }
  }

  // ────────────────────────────────────────────────────────
  // Close everything
  // ────────────────────────────────────────────────────────

  private closeAll(): void {
    this.chevIdx = -1;
    this.siblings = [];
    this.showPinDrop = false;
    this.showDriveMenu = false;
    this.closeCtx();
  }

  // ────────────────────────────────────────────────────────
  // Outside click
  // ────────────────────────────────────────────────────────

  @HostListener('document:click', ['$event'])
  onDocClick(e: MouseEvent): void {
    if (!this.elRef.nativeElement.contains(e.target as Node)) {
      this.closeAll();
      if (this.editMode) this.exitEditMode(false);
    }
  }

  @HostListener('document:keydown', ['$event'])
  onDocKey(e: KeyboardEvent): void {
    if (e.key === 'Escape') {
      if (this.ctxMenu.visible) { this.closeCtx(); e.preventDefault(); }
      else if (this.editMode) { this.exitEditMode(false); e.preventDefault(); }
      else if (this.chevIdx >= 0 || this.showPinDrop || this.showDriveMenu) { this.closeAll(); e.preventDefault(); }
    }
  }
}