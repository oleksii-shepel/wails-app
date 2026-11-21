import { ChangeDetectionStrategy, ChangeDetectorRef, Component, EventEmitter, Input, OnDestroy, OnInit, Output } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';

import { ApiService } from '../services/api.service';
import { InputEngineService } from '../services/input-engine.service';
import { QUICK_ACCESS_INPUT, QUICK_ACCESS_INPUT_COMMANDS } from '../services/dialog-input-catalog';
import { tryInjectElementRef, tryInjectModalStack } from '../services/modal-stack.service';
import { PathHistoryService } from '../services/path-history.service';
import { PinnedLocationService } from '../services/pinned-location.service';
import { RelatedLocationService } from '../services/related-location.service';
import { RemoteConnectionsService } from '../services/remote-connections.service';
import type { PinnedLocation } from './address-bar.types';
import type { SpecialFolder } from '../shared/protocol';

import { discoveryScore } from './discovery';
import { rankNavigationCandidates } from '../services/navigation-intelligence';
import { locationIdentity, locationLabel } from '../services/location-reference';

export interface QuickAccessTarget {
  path: string;
  surfaceId?: string;
  label?: string;
}

interface QuickItem {
  id: string;
  label: string;
  path: string;
  icon: string;
  kind: 'system' | 'pin' | 'frequent' | 'recent' | 'related' | 'remote' | 'smart';
  meta?: string;
  surfaceId?: string;
}

interface QuickSection {
  id: string;
  title: string;
  items: QuickItem[];
}

const CONTEXT_ID = QUICK_ACCESS_INPUT.context.id;
const OWNER_ID = 'overlay.quickAccess';
const COMMAND = QUICK_ACCESS_INPUT_COMMANDS;
const DEFAULT_LOCATION_ICON = '\u{1F4C1}';
const PINNED_LOCATION_ICON = '\u{1F4CC}';
const SPECIAL_LOCATION_ICONS: Record<string, string> = {
  home: '\u{1F3E0}',
  desktop: '\u{1F5A5}\uFE0F',
  documents: '\u{1F4C4}',
  downloads: '\u{1F4E5}',
  music: '\u{1F3B5}',
  pictures: '\u{1F5BC}\uFE0F',
  videos: '\u{1F3AC}',
};

@Component({
  selector: 'app-quick-access',
  standalone: true,
  imports: [CommonModule, FormsModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="backdrop palette-dialog-backdrop dialog-group" *ngIf="renderVisible" [class.backdrop--closing]="isClosing" [class.dialog-group--closing]="isClosing" (mousedown)="close()" (animationend)="onBackdropAnimationEnd($event)">
      <section class="quick palette-dialog-surface dialog-surface" role="dialog" aria-modal="true" aria-label="Quick Access" (mousedown)="$event.stopPropagation()"
        tabindex="-1" (keydown.escape)="close(); $event.preventDefault(); $event.stopPropagation()">
        <header class="palette-dialog-header">
          <div><strong>Quick Access</strong><span>{{ totalItems }} items</span></div>
          <input type="search" class="palette-dialog-field" [(ngModel)]="filterText" (ngModelChange)="resetSelection()" placeholder="Filter locations"
            autocomplete="off" spellcheck="false" aria-label="Filter Quick Access" aria-controls="quick-access-list" />
          <button type="button" class="dialog-close" aria-label="Close dialog" title="Close (Esc)" (click)="close()"> × </button>
        </header>

        <nav class="sections" *ngIf="filteredSections.length > 1" aria-label="Quick Access sections">
          <button *ngFor="let section of filteredSections; let i=index" type="button"
            [class.active]="i === sectionIndex" [attr.aria-pressed]="i === sectionIndex" (click)="selectSection(i)">
            {{ section.title }} <span>{{ section.items.length }}</span>
          </button>
        </nav>

        <div id="quick-access-list" class="items" role="listbox" [attr.aria-label]="(activeSection?.title || 'Quick Access') + ' locations'">
          <button *ngFor="let item of activeItems; let i=index; trackBy:trackItem" type="button"
            class="item" [class.selected]="i === itemIndex" role="option"
            [attr.aria-selected]="i === itemIndex" [attr.aria-posinset]="i + 1" [attr.aria-setsize]="activeItems.length" (mouseenter)="itemIndex=i" (click)="openItem(item)">
            <span class="icon" aria-hidden="true">
              <kbd *ngIf="i < 9">{{ i + 1 }}</kbd>
              <span class="glyph">{{ item.icon }}</span>
            </span>
            <span class="details"><strong>{{ item.label }}</strong><small [title]="item.path">{{ item.path }}</small></span>
            <span class="meta">{{ item.meta || item.kind }}</span>
          </button>
          <div class="empty" *ngIf="activeItems.length === 0" role="status">No matching locations.</div>
        </div>

        <footer>
          <span>{{ activeSection?.title || '' }}</span>
          <button type="button" [disabled]="selectedItem?.kind !== 'pin'" (click)="removeSelectedPin()">Unpin</button>
          <span class="keys">1–9 · Ctrl+PgUp/PgDn sections · ↑ ↓ · Enter · Delete · Esc</span>
        </footer>
      </section>
    </div>
  `,
  styles: [`
    :host{position:fixed;inset:0;z-index:9460;pointer-events:none}
    .backdrop{position:fixed;inset:0;display:grid;place-items:start center;padding-top:min(10vh,90px);background: var(--vsc-scrim-soft);pointer-events:auto;animation:fadeIn .2s ease}
    .backdrop--closing{animation:fadeOut .2s ease forwards;pointer-events:none}
    .quick{box-sizing:border-box;display:flex;flex-direction:column;width:min(720px,calc(100vw - 24px));max-height:min(680px,calc(100vh - 40px));overflow:hidden;border:1px solid var(--vsc-border,var(--vsc-border));border-radius:8px;background:var(--vsc-menu-background,var(--vsc-panel-background));color:var(--vsc-foreground,var(--vsc-foreground));box-shadow:0 18px 60px var(--vsc-scrim);font:12px/1.4 var(--vsc-font-family,system-ui,sans-serif);animation:slideDown .28s cubic-bezier(.16,1,.3,1)}
    .backdrop--closing .quick{animation:slideUp .2s ease forwards}
    @keyframes fadeIn{from{opacity:0}to{opacity:1}}
    @keyframes fadeOut{from{opacity:1}to{opacity:0}}
    @keyframes slideDown{from{transform:translateY(-18px);opacity:0}to{transform:translateY(0);opacity:1}}
    @keyframes slideUp{from{transform:translateY(0);opacity:1}to{transform:translateY(-18px);opacity:0}}
    header,footer{display:flex;align-items:center;gap:9px;padding:9px 10px} header{justify-content:space-between;border-bottom:1px solid var(--vsc-border-subtle,var(--vsc-border-subtle))} header>div{display:flex;gap:8px;align-items:baseline}
    header span,.meta,.keys,footer>span,small{color:var(--vsc-description-foreground,var(--vsc-foreground-dim))}
    input{box-sizing:border-box;width:min(300px,42vw)}
    .sections{display:flex;gap:2px;padding:5px;overflow-x:auto;border-bottom:1px solid var(--vsc-border-subtle,var(--vsc-border-subtle))}
    .sections button,footer button{min-height:27px;padding:3px 8px;border:1px solid transparent;border-radius:4px;background:transparent;color:inherit;cursor:pointer;font:inherit}
    .sections button{color:var(--vsc-description-foreground,var(--vsc-foreground-dim))} .sections button.active{background:var(--vsc-list-hover-background,var(--vsc-list-hover-background));color:var(--vsc-foreground,var(--vsc-foreground));border-color:var(--vsc-border,var(--vsc-border))} .sections span{margin-left:5px;opacity:.6;font-size:10px}
    .items{min-height:0;overflow:auto;padding:5px}.item{display:grid;grid-template-columns:40px minmax(0,1fr) max-content;gap:8px;align-items:center;width:100%;min-height:46px;padding:5px 8px;border:1px solid transparent;border-radius:4px;background:transparent;color:inherit;text-align:left;cursor:pointer;font:inherit}.item:hover,.item.selected{background:var(--vsc-list-hover-background,var(--vsc-list-hover-background))}.item.selected{border-color:color-mix(in srgb,var(--vsc-accent-blue,var(--vsc-accent-blue)) 50%,transparent)}
    .icon{min-width:0;overflow:hidden;display:flex;gap:5px;align-items:center;justify-content:center;text-align:center;font-size:16px}.icon kbd{flex:0 0 auto;font:9px var(--vsc-font-family-mono,ui-monospace,monospace);opacity:.55}.glyph{min-width:0;overflow:hidden;white-space:nowrap;line-height:1}.details{min-width:0;display:grid}.details strong,.details small{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.details small{font-family:var(--vsc-font-family-mono,ui-monospace,monospace)}.meta{font-size:10px;white-space:nowrap}.empty{padding:38px;text-align:center;color:var(--vsc-description-foreground,var(--vsc-foreground-dim))}
    footer{border-top:1px solid var(--vsc-border-subtle,var(--vsc-border-subtle))} footer button:hover:not(:disabled){background:var(--vsc-list-hover-background,var(--vsc-list-hover-background));border-color:var(--vsc-border,var(--vsc-border))}footer button:disabled{opacity:.35;cursor:default}.keys{margin-left:auto;font-size:10px}
  `],
})
export class QuickAccessComponent implements OnInit, OnDestroy {
  @Input() paneId: 'left' | 'right' = 'left';
  @Input() currentPath = '';
  @Input() currentSurfaceId = 'dir';
  @Output() navigate = new EventEmitter<string>();
  @Output() openTarget = new EventEmitter<QuickAccessTarget>();

  visible = false;
  renderVisible = false;
  isClosing = false;
  filterText = '';
  sections: QuickSection[] = [];
  sectionIndex = 0;
  itemIndex = 0;
  private pins: PinnedLocation[] = [];
  private readonly modalStack = tryInjectModalStack();
  private readonly hostElement = tryInjectElementRef();
  private unregisterModal: (() => void) | null = null;

  constructor(
    private readonly api: ApiService,
    private readonly history: PathHistoryService,
    private readonly pinnedLocations: PinnedLocationService,
    private readonly relatedLocations: RelatedLocationService,
    private readonly input: InputEngineService,
    private readonly connections: RemoteConnectionsService,
    private readonly cdr: ChangeDetectorRef,
  ) {}

  get filteredSections(): QuickSection[] {
    const query = this.filterText.trim();
    if (!query) return this.sections;

    return this.sections
      .map(section => ({
        ...section,
        items: section.items
          .map(item => ({
            item,
            score: discoveryScore(query, [
              { value: item.label, weight: 1.5 },
              { value: item.path, weight: 1.1 },
              { value: item.meta, weight: .7 },
              { value: item.kind, weight: .6 },
            ]),
          }))
          .filter(candidate => candidate.score >= 0)
          .sort(
            (a,b) =>
              b.score - a.score ||
              a.item.label.localeCompare(b.item.label),
          )
          .map(candidate => candidate.item),
      }))
      .filter(section => section.items.length > 0);
  }

  get activeSection(): QuickSection | null { return this.filteredSections[this.sectionIndex] ?? null; }
  get activeItems(): QuickItem[] { return this.activeSection?.items ?? []; }
  get selectedItem(): QuickItem | null { return this.activeItems[this.itemIndex] ?? null; }
  get totalItems(): number { return this.filteredSections.reduce((sum,s) => sum + s.items.length, 0); }

  ngOnInit(): void {
    this.input.registerCommandDescriptor({id:COMMAND.close,title:'Close Quick Access',category:'Navigation',execute:()=>this.close()},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.previous,title:'Previous Quick Access Item',category:'Navigation',execute:()=>this.moveItem(-1)},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.next,title:'Next Quick Access Item',category:'Navigation',execute:()=>this.moveItem(1)},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.previousSection,title:'Previous Quick Access Section',category:'Navigation',execute:()=>this.moveSection(-1)},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.nextSection,title:'Next Quick Access Section',category:'Navigation',execute:()=>this.moveSection(1)},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.open,title:'Open Quick Access Item',category:'Navigation',execute:()=>{const item=this.selectedItem;if(item)this.openItem(item)}},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.removePin,title:'Remove Quick Access Pin',category:'Navigation',execute:()=>this.removeSelectedPin(),isEnabled:()=>this.selectedItem?.kind==='pin'},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.first,title:'First Quick Access Item',category:'Navigation',execute:()=>this.jumpBoundary('first')},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.last,title:'Last Quick Access Item',category:'Navigation',execute:()=>this.jumpBoundary('last')},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.openSlot1,title:'Open Quick Access Slot 1',category:'Navigation',execute:()=>this.openSlot(0),isEnabled:()=>this.activeItems.length>0},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.openSlot2,title:'Open Quick Access Slot 2',category:'Navigation',execute:()=>this.openSlot(1),isEnabled:()=>this.activeItems.length>1},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.openSlot3,title:'Open Quick Access Slot 3',category:'Navigation',execute:()=>this.openSlot(2),isEnabled:()=>this.activeItems.length>2},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.openSlot4,title:'Open Quick Access Slot 4',category:'Navigation',execute:()=>this.openSlot(3),isEnabled:()=>this.activeItems.length>3},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.openSlot5,title:'Open Quick Access Slot 5',category:'Navigation',execute:()=>this.openSlot(4),isEnabled:()=>this.activeItems.length>4},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.openSlot6,title:'Open Quick Access Slot 6',category:'Navigation',execute:()=>this.openSlot(5),isEnabled:()=>this.activeItems.length>5},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.openSlot7,title:'Open Quick Access Slot 7',category:'Navigation',execute:()=>this.openSlot(6),isEnabled:()=>this.activeItems.length>6},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.openSlot8,title:'Open Quick Access Slot 8',category:'Navigation',execute:()=>this.openSlot(7),isEnabled:()=>this.activeItems.length>7},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.openSlot9,title:'Open Quick Access Slot 9',category:'Navigation',execute:()=>this.openSlot(8),isEnabled:()=>this.activeItems.length>8},OWNER_ID);
  }

  ngOnDestroy(): void {
    this.unregisterModal?.();
    this.unregisterModal = null;
    this.input.deactivateContext(CONTEXT_ID);
    for (const id of Object.values(COMMAND)) this.input.unregisterCommandDescriptor(id, OWNER_ID);
  }

  async open(sectionId?: string): Promise<void> {
    
    await this.reload();

    this.filterText = '';
    this.itemIndex = 0;

    if (sectionId) {
      const index = this.sections.findIndex(section => section.id === sectionId);
      this.sectionIndex = index >= 0 ? index : 0;
    } else {
      this.sectionIndex = 0;
    }

    this.visible = true;
    this.renderVisible = true;
    this.isClosing = false;
    this.input.activateContext(CONTEXT_ID);
    if (this.modalStack && this.hostElement && !this.unregisterModal) {
      this.unregisterModal = this.modalStack.register({
        id: 'quick-access',
        element: this.hostElement.nativeElement,
        contextId: CONTEXT_ID,
        dismiss: () => this.close(),
        dismissible: () => this.renderVisible && !this.isClosing,
      });
    }
    this.cdr.markForCheck();
  }

  close(): void {
    if(!this.renderVisible||this.isClosing)return;
    this.visible=false;
    this.isClosing=true;
    this.unregisterModal?.();
    this.unregisterModal = null;
    this.input.deactivateContext(CONTEXT_ID);
    this.cdr.markForCheck();
  }
  openItem(item: QuickItem): void {
    const surfaceId = item.surfaceId?.trim();

    if (surfaceId && surfaceId !== 'dir') {
      this.openTarget.emit({
        path: item.path,
        surfaceId,
        label: item.label,
      });
    } else {
      this.navigate.emit(item.path);
    }

    this.close();
  }
  selectSection(index:number):void{this.sectionIndex=Math.max(0,Math.min(index,this.filteredSections.length-1));this.itemIndex=0;this.cdr.markForCheck()}
  resetSelection():void{this.sectionIndex=0;this.itemIndex=0;this.cdr.markForCheck()}
  trackItem(_index:number,item:QuickItem):string{return item.id}

  removeSelectedPin(): void {
    const item=this.selectedItem; if(!item||item.kind!=='pin')return;
    this.pins = this.pinnedLocations.remove(this.pins, item.path, item.surfaceId);
    this.pinnedLocations.save(this.pins);
    void this.reload();
  }

  private async reload(): Promise<void> {
    const [specialFolders, drivesResponse, remoteMounts] = await Promise.all([
      this.api.getSpecialFolders().catch(() => [] as SpecialFolder[]),
      this.api.getDrives().catch(() => ({ schemes: [] })),
      this.connections.refresh().catch(() => this.connections.mounts),
    ]);
    this.pins = this.pinnedLocations.load();

    const allHistory = this.history.getAllHistory();
    const paneHistory = allHistory.filter(entry => entry.paneId === this.paneId);
    const historyPool = paneHistory.length > 0 ? paneHistory : allHistory;

    const currentId = this.locationKey(this.currentPath);

    const frequent = [...historyPool]
      .filter(entry => entry.accessCount >= 2)
      .sort((a, b) =>
        b.accessCount - a.accessCount || b.timestamp - a.timestamp,
      )
      .filter(entry => this.locationKey(entry.location) !== currentId)
      .slice(0, 8);

    const recent = [...historyPool]
      .sort((a, b) => b.timestamp - a.timestamp)
      .filter(entry => this.locationKey(entry.location) !== currentId)
      .slice(0, 10);

    const historyRelated = this.currentPath
      ? this.history.getRelatedLocations(this.currentPath, 8)
        .filter(entry => this.locationKey(entry.location) !== currentId)
      : [];

    const contributedRelated = this.currentPath
      ? this.relatedLocations.resolve(
          this.pins,
          this.currentPath,
          this.currentSurfaceId,
        )
      : [];

    const unique = (items: QuickItem[]): QuickItem[] => {
      const seen = new Set<string>();
      return items.filter(item => {
        const location = this.locationKey(item.path);
        const key = `${location}|${item.surfaceId || 'dir'}`;
        if (!location || seen.has(key)) return false;
        seen.add(key);
        return true;
      });
    };

    const system: QuickItem[] = specialFolders.map(folder => ({
      id: `system:${folder.path}`,
      label: folder.name,
      path: folder.path,
      icon: this.iconFor(folder.icon, DEFAULT_LOCATION_ICON),
      kind: 'system',
    }));

    const driveItems: QuickItem[] = (drivesResponse.schemes ?? []).map(drive => ({
      id: `drive:${drive.path}`,
      label: drive.name || drive.path,
      path: drive.path,
      icon:
        drive.schemeType === 'removable' ? '⏏' :
        drive.schemeType === 'network' ? '⇄' :
        drive.schemeType === 'cdrom' ? '◉' :
        drive.schemeType === 'fixed' ? '▰' :
        '◆',
      kind: 'system',
      meta: String(drive.schemeType || '').toUpperCase(),
    }));

    const pins: QuickItem[] = this.pins.map(pin => ({
      id: `pin:${pin.id}`,
      label: pin.label,
      path: pin.location,
      icon: this.iconFor(pin.icon, PINNED_LOCATION_ICON),
      kind: 'pin',
      surfaceId: pin.surfaceId,
      meta:
        pin.surfaceId && pin.surfaceId !== 'dir'
          ? pin.surfaceId
          : undefined,
    }));

    const relatedItems: QuickItem[] = [
      ...contributedRelated.map(pin => ({
        id: `related-pin:${pin.id}`,
        label: pin.label,
        path: pin.location,
        icon: this.iconFor(pin.icon, '◆'),
        meta: pin.surfaceId || pin.scheme,
        surfaceId: pin.surfaceId,
        kind: 'related' as const,
      })),
      ...historyRelated.map(entry => ({
        id: `related:${entry.location}`,
        label: this.labelFor(entry.location),
        path: entry.location,
        icon: '↔',
        meta: entry.scheme,
        kind: 'related' as const,
      })),
    ];

    const frequentItems: QuickItem[] = frequent.map(entry => ({
      id: `frequent:${entry.location}`,
      label: this.labelFor(entry.location),
      path: entry.location,
      icon: '↻',
      meta: `×${entry.accessCount}`,
      kind: 'frequent',
    }));

    const recentItems: QuickItem[] = recent.map(entry => ({
      id: `recent:${entry.location}`,
      label: this.labelFor(entry.location),
      path: entry.location,
      icon: '◷',
      meta: this.relativeTime(entry.timestamp),
      kind: 'recent',
    }));

    const remoteItems: QuickItem[] = remoteMounts
      .map(mount => ({
        id: `remote:${this.connections.mountPoint(mount) || this.connections.rootUri(mount)}`,
        label: this.connections.label(mount),
        path: this.connections.rootUri(mount),
        icon: '☁',
        kind: 'remote' as const,
        meta: (this.connections.protocol(mount) || 'remote').toUpperCase(),
      }))
      .filter(item => !!item.path);

    const smartPool = unique([...pins, ...frequentItems, ...recentItems, ...relatedItems, ...remoteItems]);
    const historyByPath = new Map(
      this.history.getAllHistory().map(entry => [this.locationKey(entry.location), entry]),
    );
    const pinnedKeys = new Set(pins.map(item => this.locationKey(item.path)));
    const smartItems: QuickItem[] = rankNavigationCandidates(
      smartPool.map(item => {
        const history = historyByPath.get(this.locationKey(item.path));
        return {
          ...item,
          kind: 'smart' as const,
          accessCount: history?.accessCount ?? 0,
          timestamp: history?.timestamp ?? 0,
          pinned: pinnedKeys.has(this.locationKey(item.path)),
          current: this.locationKey(item.path) === this.locationKey(this.currentPath),
        };
      }),
      this.currentPath,
    ).slice(0,12).map(candidate => ({
      ...candidate.item,
      kind: 'smart' as const,
      meta: candidate.reasons.slice(0,2).join(' · ') || candidate.item.meta,
    }));

    this.sections = [
      ...(smartItems.length ? [{ id: 'smart', title: 'Smart', items: smartItems }] : []),
      ...(remoteItems.length ? [{ id: 'remote', title: 'Remote', items: remoteItems }] : []),
      { id: 'drives', title: 'Drives & Roots', items: unique(driveItems) },
      { id: 'system', title: 'System', items: unique(system) },
      { id: 'pins', title: 'Pinned', items: unique(pins) },
      { id: 'related', title: 'Related', items: unique(relatedItems) },
      { id: 'frequent', title: 'Frequent', items: unique(frequentItems) },
      { id: 'recent', title: 'Recent', items: unique(recentItems) },
    ].filter(section => section.items.length > 0);

    this.sectionIndex = Math.min(
      this.sectionIndex,
      Math.max(0, this.sections.length - 1),
    );
    this.itemIndex = 0;
    this.cdr.markForCheck();
  }

  private openSlot(index: number): void {
    const item = this.activeItems[index];
    if (item) this.openItem(item);
  }

  private jumpBoundary(boundary: 'first' | 'last'): void {
    const count = this.activeItems.length;
    if (!count) return;
    this.itemIndex = boundary === 'first' ? 0 : count - 1;
    this.cdr.markForCheck();
  }

  private relativeTime(timestamp: number): string {
    const delta = Math.max(0, Date.now() - timestamp);
    if (delta < 60_000) return 'now';
    if (delta < 3_600_000) return `${Math.floor(delta / 60_000)}m`;
    if (delta < 86_400_000) return `${Math.floor(delta / 3_600_000)}h`;
    if (delta < 604_800_000) return `${Math.floor(delta / 86_400_000)}d`;
    return new Date(timestamp).toLocaleDateString();
  }

  private locationKey(path: string): string {
    return locationIdentity(path);
  }

  private moveItem(delta:-1|1):void{const count=this.activeItems.length;if(!count)return;this.itemIndex=(this.itemIndex+delta+count)%count;this.cdr.markForCheck()}
  private moveSection(delta:-1|1):void{const count=this.filteredSections.length;if(!count)return;this.sectionIndex=(this.sectionIndex+delta+count)%count;this.itemIndex=0;this.cdr.markForCheck()}
  private iconFor(icon:string|undefined,fallback:string):string{const normalized=(icon||'').trim();if(!normalized)return fallback;return SPECIAL_LOCATION_ICONS[normalized.toLowerCase()]??normalized}
  private labelFor(path: string): string { return locationLabel(path) || path; }
  onBackdropAnimationEnd(event: AnimationEvent): void {
    if (event.target !== event.currentTarget || !this.isClosing) return;
    this.renderVisible = false;
    this.isClosing = false;
    this.cdr.markForCheck();
  }
}