import {
  ChangeDetectionStrategy,
  ChangeDetectorRef,
  Component,
  ElementRef,
  HostListener,
  Input,
  OnDestroy,
  OnInit,
  ViewChild,
} from '@angular/core';
import { CommonModule } from '@angular/common';

import { CommandPaletteHistoryService } from '../services/command-palette-history.service';
import { InputEngineService } from '../services/input-engine.service';
import type { InputBindingDefinition } from '../services/input-system.types';

export interface PaletteCommand {
  id: string;
  title: string;
  category: string;
  shortcut?: string;
  execute: () => void | Promise<void>;
  isEnabled?: () => boolean;
}

import { discoveryScore } from './discovery';
const CONTEXT_ID = 'overlay.CommandPalette';
const OWNER_ID = 'overlay.CommandPalette';

const COMMAND = {
  close: 'reassembly.commandPalette.close',
  previous: 'reassembly.commandPalette.previous',
  next: 'reassembly.commandPalette.next',
  run: 'reassembly.commandPalette.run',
} as const;

const BINDINGS: readonly InputBindingDefinition[] = [
  { id:'reassembly.commandPalette.escape', commandId:COMMAND.close, contextId:CONTEXT_ID, source:'default', description:'Close command palette', category:'Navigation', trigger:{device:'keyboard', chord:{steps:[{key:'escape'}]}}},
  { id:'reassembly.commandPalette.up', commandId:COMMAND.previous, contextId:CONTEXT_ID, source:'default', description:'Previous command', category:'Navigation', trigger:{device:'keyboard', chord:{steps:[{key:'arrowup'}]}}},
  { id:'reassembly.commandPalette.down', commandId:COMMAND.next, contextId:CONTEXT_ID, source:'default', description:'Next command', category:'Navigation', trigger:{device:'keyboard', chord:{steps:[{key:'arrowdown'}]}}},
  { id:'reassembly.commandPalette.enter', commandId:COMMAND.run, contextId:CONTEXT_ID, source:'default', description:'Run command', category:'Navigation', trigger:{device:'keyboard', chord:{steps:[{key:'enter'}]}}},
];

@Component({
  selector: 'app-command-palette',
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="backdrop dialog-group" *ngIf="visible || isClosing" [class.backdrop-closing]="isClosing" [class.dialog-group--closing]="isClosing" (mousedown)="close()"
      (animationend)="onAnimationEnd($event)">
      <section class="palette dialog-surface" role="dialog" aria-modal="true" aria-label="Command palette"
        (mousedown)="$event.stopPropagation()"
        tabindex="-1" (keydown.escape)="close(); $event.preventDefault(); $event.stopPropagation()">
        <header>
          <div class="title-block">
            <strong>Command Palette</strong>
            <span>{{ filteredCommands.length }} command{{ filteredCommands.length === 1 ? '' : 's' }}</span>
          </div>
          <span class="dialog-close" aria-label="Close tab" title="Close" (click)="close()" role="button" tabindex="0" (keydown.enter)="close()" (keydown.space)="close(); $event.preventDefault()"> × </span>
        </header>

        <div class="query">
          <span aria-hidden="true">›</span>
          <input #queryInput [value]="query" (input)="onQueryInput($event)"
            autocomplete="off" spellcheck="false"
            placeholder="Type a command, category, shortcut, or id"
            aria-label="Command search" aria-controls="command-list" />
        </div>

        <nav class="categories" *ngIf="categories.length > 1" aria-label="Command category filters">
          <button
            type="button"
            [class.active]="selectedCategory === 'all'"
            (click)="selectCategory('all')">
            All
          </button>
          <button
            *ngFor="let category of categories"
            type="button"
            [class.active]="selectedCategory === category"
            (click)="selectCategory(category)">
            {{ category }}
          </button>
        </nav>

        <div id="command-list" class="commands" role="listbox" aria-label="Commands">
          <button *ngFor="let command of filteredCommands; let i=index; trackBy:trackCommand"
            type="button" class="command" role="option"
            [class.selected]="i === selectedIndex"
            [class.disabled]="!enabled(command)"
            [attr.aria-selected]="i === selectedIndex"
            [attr.aria-posinset]="i + 1"
            [attr.aria-setsize]="filteredCommands.length"
            [attr.aria-disabled]="!enabled(command)"
            (mouseenter)="selectedIndex=i"
            (click)="run(command)">
            <span class="main">
              <strong>{{ command.title }}</strong>
              <small>{{ isRecent(command.id) ? 'Recent · ' : '' }}{{ command.category }} · {{ command.id }}</small>
            </span>
            <kbd *ngIf="command.shortcut">{{ command.shortcut }}</kbd>
            <span class="state" *ngIf="!enabled(command)">Unavailable</span>
          </button>

          <div class="empty" *ngIf="filteredCommands.length === 0">No matching commands.</div>
        </div>

        <footer>
          <span>↑ ↓ navigate · Enter run</span>
          <span>Ctrl+Shift+P</span>
        </footer>
      </section>
    </div>
  `,
  styles: [`
    :host{position:fixed;inset:0;z-index:9520;pointer-events:none}
    .backdrop{position:fixed;inset:0;display:grid;place-items:start center;padding-top:min(8vh,72px);background: var(--vsc-scrim-soft);pointer-events:auto}
    .backdrop-closing{pointer-events:none}
    .palette{box-sizing:border-box;display:flex;flex-direction:column;width:min(760px,calc(100vw - 24px));height:min(620px,calc(100vh - 40px));overflow:hidden;border:1px solid var(--vsc-border);border-radius:8px;background:var(--vsc-menu-background,var(--vsc-panel-background));color:var(--vsc-foreground);box-shadow:0 18px 62px var(--vsc-scrim-strong);font:12px/1.4 var(--vsc-font-family,system-ui,sans-serif)}
    header,footer,.query{display:flex;align-items:center;gap:10px;padding:9px 11px;border-bottom:1px solid var(--vsc-border-subtle)}
    header{justify-content:space-between}
    .title-block{display:flex;align-items:baseline;gap:8px;min-width:0;font-variant-numeric:tabular-nums}
    header span,footer,.command small,.state{color:var(--vsc-description-foreground,var(--vsc-foreground-dim))}
    .query{font-size:18px}.query input{flex:1;height:34px;border:0;outline:0;background:transparent;color:inherit;font:13px var(--vsc-font-family,system-ui,sans-serif)}
    .categories{display:flex;align-items:center;gap:4px;overflow-x:auto;padding:6px 10px;border-bottom:1px solid var(--vsc-border-subtle)}
    .categories button{padding:3px 8px;border:1px solid transparent;border-radius:4px;background:transparent;color:var(--vsc-description-foreground,var(--vsc-foreground-dim));font:inherit;font-size:11px;cursor:pointer;white-space:nowrap}
    .categories button:hover{color:var(--vsc-foreground);background:var(--vsc-list-hover-background)}
    .categories button.active{color:var(--vsc-foreground);border-color:color-mix(in srgb,var(--vsc-accent-blue) 55%,var(--vsc-border));background:color-mix(in srgb,var(--vsc-accent-blue) 14%,transparent)}
    .commands{flex:1;min-height:0;overflow:auto;padding:5px}
    .command{display:grid;grid-template-columns:minmax(0,1fr) auto auto;align-items:center;gap:10px;width:100%;min-height:48px;padding:6px 9px;border:1px solid transparent;border-radius:4px;background:transparent;color:inherit;text-align:left;cursor:pointer;font:inherit}
    .command:hover,.command.selected{background:var(--vsc-list-hover-background)}
    .command.selected{border-color:color-mix(in srgb,var(--vsc-accent-blue) 55%,transparent)}
    .command.disabled{opacity:.48}.main{min-width:0;display:grid}.main strong,.main small{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
    .main small{font:10px var(--vsc-font-family-mono,ui-monospace,monospace)}
    kbd{padding:2px 6px;border:1px solid var(--vsc-border);border-radius:4px;background:var(--vsc-neutral-overlay-subtle);font:10px var(--vsc-font-family-mono,ui-monospace,monospace);white-space:nowrap}
    .state{font-size:10px}.empty{padding:42px;text-align:center;color:var(--vsc-description-foreground,var(--vsc-foreground-dim))}
    footer{justify-content:space-between;border-top:1px solid var(--vsc-border-subtle);border-bottom:0}
    .dialog-close{margin-left:8px}
  `],
})
export class CommandPaletteComponent implements OnInit, OnDestroy {
  @ViewChild('queryInput') queryInput?: ElementRef<HTMLInputElement>;
  @Input() commands: readonly PaletteCommand[] = [];

  visible = false;
  isClosing = false;
  query = '';
  selectedCategory = 'all';
  selectedIndex = 0;

  constructor(
    private readonly history: CommandPaletteHistoryService,
    private readonly input: InputEngineService,
    private readonly cdr: ChangeDetectorRef,
  ) {}

  get categories(): string[] {
    const unique = new Set<string>();
    for (const command of this.commands) {
      if (command.category) unique.add(command.category);
    }
    return Array.from(unique).sort((a, b) => a.localeCompare(b));
  }

  get filteredCommands(): PaletteCommand[] {
    const query = this.query.trim();
    const recentIds = this.history.getHistory();
    const recentRank = new Map<string, number>();
    recentIds.forEach((id, idx) => recentRank.set(id, idx));

    const scoped =
      this.selectedCategory === 'all'
        ? [...this.commands]
        : this.commands.filter(command => command.category === this.selectedCategory);

    if (!query) {
      return scoped.sort((a, b) => {
        const ra = recentRank.get(a.id);
        const rb = recentRank.get(b.id);
        if (ra !== undefined && rb !== undefined) return ra - rb;
        if (ra !== undefined) return -1;
        if (rb !== undefined) return 1;
        return a.category.localeCompare(b.category) || a.title.localeCompare(b.title);
      });
    }

    return scoped
      .map(command => ({
        command,
        score:
          discoveryScore(query, [
            { value: command.title, weight: 1.6 },
            { value: command.category, weight: 1.1 },
            { value: command.shortcut, weight: 1.25 },
            { value: command.id, weight: 0.8 },
          ]) + (recentRank.has(command.id) ? 0.15 : 0),
      }))
      .filter(item => item.score >= 0)
      .sort(
        (a, b) =>
          b.score - a.score ||
          Number(this.enabled(b.command)) - Number(this.enabled(a.command)) ||
          a.command.title.localeCompare(b.command.title),
      )
      .map(item => item.command);
  }

  get selectedCommand(): PaletteCommand | null {
    return this.filteredCommands[this.selectedIndex] ?? null;
  }

  isRecent(commandId: string): boolean {
    return this.history.getHistory().includes(commandId);
  }

  ngOnInit(): void {
    this.input.registerContext({
      id: CONTEXT_ID,
      kind: 'overlay',
      priority: 410, exclusive: true,
      parentId: 'global',
      description: 'Reassembly command palette',
    });

    this.input.registerCommandDescriptor({id:COMMAND.close,title:'Close Command Palette',category:'Navigation',execute:()=>this.close()},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.previous,title:'Previous Command',category:'Navigation',execute:()=>this.move(-1)},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.next,title:'Next Command',category:'Navigation',execute:()=>this.move(1)},OWNER_ID);
    this.input.registerCommandDescriptor({id:COMMAND.run,title:'Run Command',category:'Navigation',execute:()=>{const command=this.selectedCommand;if(command)void this.run(command)}},OWNER_ID);

    for (const binding of BINDINGS) this.input.registerBinding(binding);
  }

  ngOnDestroy(): void {
    this.input.deactivateContext(CONTEXT_ID);
    for (const binding of BINDINGS) this.input.unregisterBinding(binding.id);
    for (const id of Object.values(COMMAND)) this.input.unregisterCommandDescriptor(id, OWNER_ID);
  }

  open(): void {
    this.query = '';
    this.selectedCategory = 'all';
    this.selectedIndex = 0;
    this.isClosing = false;
    this.visible = true;
    this.input.activateContext(CONTEXT_ID);
    this.cdr.markForCheck();
    requestAnimationFrame(() => this.queryInput?.nativeElement.focus({ preventScroll: true }));
  }

  close(): void {
    if (!this.visible || this.isClosing) return;
    this.isClosing = true;
    this.input.deactivateContext(CONTEXT_ID);
    this.cdr.markForCheck();
  }

  onQueryInput(event: Event): void {
    this.query = (event.target as HTMLInputElement).value;
    this.queryChanged();
  }

  selectCategory(category: string): void {
    this.selectedCategory = category;
    this.selectedIndex = 0;
    this.cdr.markForCheck();
  }

  queryChanged(): void {
    this.selectedIndex = 0;
    this.cdr.markForCheck();
  }

  enabled(command: PaletteCommand): boolean {
    try { return command.isEnabled ? command.isEnabled() : true; }
    catch { return false; }
  }

  async run(command: PaletteCommand): Promise<void> {
    if (!this.enabled(command)) return;
    this.history.recordCommand(command.id);
    this.close();
    await command.execute();
  }

  trackCommand(_index:number, command:PaletteCommand):string { return command.id; }

  private move(delta:-1|1):void {
    const commands = this.filteredCommands;
    if (!commands.length) return;

    let index = this.selectedIndex;
    for (let attempts=0; attempts<commands.length; attempts+=1) {
      index = (index + delta + commands.length) % commands.length;
      if (this.enabled(commands[index])) {
        this.selectedIndex = index;
        this.cdr.markForCheck();
        return;
      }
    }
  }

  @HostListener('document:keydown', ['$event'])
  onDocumentKeydown(event: KeyboardEvent): void {
    if (!this.visible || this.isClosing || event.key !== 'Escape') return;
    event.preventDefault();
    event.stopPropagation();
    this.close();
  }

  onAnimationEnd(event: AnimationEvent): void {
    if (event.target !== event.currentTarget || !this.isClosing) return;
    this.visible = false;
    this.isClosing = false;
    this.cdr.markForCheck();
  }

}