import { CommonModule } from '@angular/common';
import {
  ChangeDetectionStrategy,
  ChangeDetectorRef,
  Component,
  ElementRef,
  Input,
  OnChanges,
  ViewChild,
  inject,
} from '@angular/core';
import { ApiService } from '../../services/api.service';
import { ActionFeedbackService } from '../../services/action-feedback.service';
import { WorkspaceService } from '../../services/workspace.service';
import type { SurfaceContext } from '../surface-context';

interface AnsiSegment {
  text: string;
  bold: boolean;
  dim: boolean;
  color: '' | 'red' | 'green' | 'yellow' | 'blue' | 'magenta' | 'cyan' | 'gray';
}

interface CliTranscriptEntry {
  id: number;
  cwd: string;
  command: string;
  segments: AnsiSegment[];
  stderrSegments: AnsiSegment[];
  exitCode: number;
  durationMs: number;
}

const KNOWN_COMMANDS = [
  'ls',
  'du',
  'find',
  'dup',
  'hash',
  'verify',
  'diff',
  'compare',
  'split',
  'join',
  'rename',
  'archive',
  'trash',
  'ws',
  'doctor',
  'cd',
  'pwd',
  'clear',
  'help',
];

const MUTATING_COMMANDS = new Set(['split', 'join', 'rename', 'trash', 'archive']);

function parseAnsiText(input: string): AnsiSegment[] {
  if (!input) return [];
  const pattern = /\x1b\[([0-9;]*)m/g;
  const segments: AnsiSegment[] = [];
  let bold = false;
  let dim = false;
  let color: AnsiSegment['color'] = '';
  let lastIndex = 0;

  let match: RegExpExecArray | null;
  while ((match = pattern.exec(input)) !== null) {
    if (match.index > lastIndex) {
      segments.push({
        text: input.slice(lastIndex, match.index),
        bold,
        dim,
        color,
      });
    }
    const codes = (match[1] || '0').split(';').map((c) => Number(c || 0));
    for (const code of codes) {
      if (code === 0) {
        bold = false;
        dim = false;
        color = '';
      } else if (code === 1) {
        bold = true;
      } else if (code === 2) {
        dim = true;
      } else if (code === 22) {
        bold = false;
        dim = false;
      } else if (code === 31) color = 'red';
      else if (code === 32) color = 'green';
      else if (code === 33) color = 'yellow';
      else if (code === 34) color = 'blue';
      else if (code === 35) color = 'magenta';
      else if (code === 36) color = 'cyan';
      else if (code === 90) color = 'gray';
      else if (code === 39) color = '';
    }
    lastIndex = pattern.lastIndex;
  }

  if (lastIndex < input.length) {
    segments.push({
      text: input.slice(lastIndex),
      bold,
      dim,
      color,
    });
  }
  return segments;
}

@Component({
  selector: 'app-cli-surface-tab',
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <section
      class="cli-pane"
      tabindex="-1"
      (click)="focusPrompt()">
      <header class="cli-header">
        <div class="cli-title-group">
          <span class="cli-badge" aria-hidden="true">&gt;_</span>
          <strong>Aether CLI</strong>
          <code class="cli-cwd" [title]="cwd">{{ cwd }}</code>
        </div>

        <div class="cli-header-actions">
          @if (peerDirectoryPath) {
            <button
              type="button"
              class="cli-btn"
              title="Set CLI working directory to the other pane's active folder"
              (click)="syncCwdFromPeer(); $event.stopPropagation()">
              Sync Peer
            </button>
          }
          <button
            type="button"
            class="cli-btn"
            title="Run environment & tool diagnostics"
            (click)="runQuickCommand('doctor'); $event.stopPropagation()">
            Doctor
          </button>
          <button
            type="button"
            class="cli-btn"
            title="Show CLI help"
            (click)="runQuickCommand('--help'); $event.stopPropagation()">
            Help
          </button>
          <button
            type="button"
            class="cli-btn"
            title="Clear transcript (Ctrl+L)"
            (click)="clearTranscript(); $event.stopPropagation()">
            Clear
          </button>
        </div>
      </header>

      <div class="cli-quickbar" aria-label="Quick CLI commands">
        <button type="button" class="cli-chip" (click)="runQuickCommand('ls -l'); $event.stopPropagation()">ls -l</button>
        <button type="button" class="cli-chip" (click)="runQuickCommand('du --top 12'); $event.stopPropagation()">du</button>
        <button type="button" class="cli-chip" (click)="prefillCommand('find -g '); $event.stopPropagation()">find -g</button>
        <button type="button" class="cli-chip" (click)="runQuickCommand('dup'); $event.stopPropagation()">dup</button>
        <button type="button" class="cli-chip" (click)="prefillCommand('hash --manifest '); $event.stopPropagation()">hash</button>
        @if (peerDirectoryPath) {
          <button type="button" class="cli-chip" (click)="compareWithPeer(); $event.stopPropagation()">compare peer</button>
        }
        <button type="button" class="cli-chip" (click)="runQuickCommand('trash list'); $event.stopPropagation()">trash</button>
        <button type="button" class="cli-chip" (click)="runQuickCommand('ws paths'); $event.stopPropagation()">ws</button>
      </div>

      <div #scrollArea class="cli-transcript" role="log" aria-live="polite" aria-label="CLI output">
        <div class="cli-banner">
          <div>
            <strong>Reassembly Command Line</strong> — running inside the {{ ctx.paneId }} pane tab.
          </div>
          <div class="cli-banner-sub">
            Type any <code>aether</code> subcommand (<code>ls</code>, <code>du</code>, <code>find</code>, <code>dup</code>, <code>hash</code>, <code>verify</code>, <code>diff</code>, <code>compare</code>, <code>split</code>, <code>join</code>, <code>rename</code>, <code>archive</code>, <code>trash</code>, <code>ws</code>, <code>doctor</code>) or <code>cd</code> / <code>pwd</code> / <code>clear</code>. Press <kbd>Tab</kbd> to complete and <kbd>↑</kbd>/<kbd>↓</kbd> for history.
          </div>
        </div>

        @for (entry of transcript; track entry.id) {
          <article class="cli-entry" [class.cli-entry--error]="entry.exitCode !== 0">
            <div class="cli-entry-cmd">
              <span class="cli-prompt-cwd">{{ shortCwd(entry.cwd) }}</span>
              <span class="cli-prompt-sigil">$</span>
              <span class="cli-prompt-text">{{ entry.command }}</span>
              <span class="cli-entry-meta">
                @if (entry.exitCode !== 0) {
                  <span class="cli-exit-fail">exit {{ entry.exitCode }}</span>
                }
                <span class="cli-duration">{{ entry.durationMs }}ms</span>
              </span>
            </div>

            @if (entry.segments.length > 0) {
              <pre class="cli-output">@for (seg of entry.segments; track $index) {<span [class]="segmentClass(seg)">{{ seg.text }}</span>}</pre>
            }

            @if (entry.stderrSegments.length > 0) {
              <pre class="cli-output cli-output--err">@for (seg of entry.stderrSegments; track $index) {<span [class]="segmentClass(seg)">{{ seg.text }}</span>}</pre>
            }
          </article>
        }

        @if (running) {
          <div class="cli-running">Executing <code>{{ activeCommandLabel }}</code>…</div>
        }
      </div>

      <footer class="cli-input-bar" (click)="$event.stopPropagation()">
        <span class="cli-prompt-label" [title]="cwd">{{ shortCwd(cwd) }} $</span>
        <input
          #cmdInput
          type="text"
          class="cli-input"
          [value]="commandInput"
          [disabled]="running"
          (input)="onCommandInput($event)"
          (keydown.enter)="submitCommand()"
          (keydown.arrowup)="historyUp($event)"
          (keydown.arrowdown)="historyDown($event)"
          (keydown.tab)="completeInput($event)"
          (keydown.control.l)="clearTranscript($event)"
          autocomplete="off"
          spellcheck="false"
          placeholder="aether ls -l, find -g pattern, diff a b, du --top 10, doctor…"
          aria-label="Aether CLI command input" />
        <button
          type="button"
          class="cli-run-btn"
          [disabled]="running || !commandInput.trim()"
          (click)="submitCommand()">
          {{ running ? 'Running…' : 'Run' }}
        </button>
      </footer>
    </section>
  `,
  styles: [`
    :host {
      display: flex;
      flex-direction: column;
      flex: 1 1 0%;
      min-height: 0;
      min-width: 0;
      height: 100%;
      background: var(--vsc-editor-background);
      color: var(--vsc-foreground);
      font-family: var(--vsc-font-family, system-ui, sans-serif);
    }

    .cli-pane {
      display: flex;
      flex-direction: column;
      flex: 1 1 0%;
      min-height: 0;
      min-width: 0;
      outline: none;
    }

    .cli-header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 10px;
      padding: 8px 12px;
      background: var(--vsc-toolbar-background, var(--vsc-panel-background));
      border-bottom: 1px solid var(--vsc-border-subtle, var(--vsc-border));
      flex-wrap: wrap;
    }

    .cli-title-group {
      display: flex;
      align-items: center;
      gap: 8px;
      min-width: 0;
      flex: 1 1 200px;
      font-size: 12px;
    }

    .cli-badge {
      display: inline-flex;
      align-items: center;
      justify-content: center;
      padding: 1px 6px;
      border-radius: 4px;
      border: 1px solid var(--vsc-border);
      background: var(--vsc-input-background);
      color: var(--vsc-accent-blue);
      font-family: var(--vsc-font-family-mono, ui-monospace, monospace);
      font-size: 11px;
      font-weight: 700;
    }

    .cli-cwd {
      min-width: 0;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
      font-family: var(--vsc-font-family-mono, ui-monospace, monospace);
      font-size: 11px;
    }

    .cli-header-actions {
      display: flex;
      align-items: center;
      gap: 6px;
      flex-wrap: wrap;
    }

    .cli-btn,
    .cli-run-btn {
      padding: 4px 9px;
      border: 1px solid var(--vsc-border);
      border-radius: 4px;
      background: var(--vsc-input-background, var(--vsc-toolbar-background));
      color: var(--vsc-foreground);
      font-size: 11px;
      cursor: pointer;
    }

    .cli-btn:hover:not(:disabled),
    .cli-run-btn:hover:not(:disabled) {
      background: var(--vsc-list-hover-background);
      border-color: var(--vsc-accent-blue);
    }

    .cli-btn:disabled,
    .cli-run-btn:disabled {
      opacity: 0.45;
      cursor: default;
    }

    .cli-quickbar {
      display: flex;
      align-items: center;
      gap: 6px;
      padding: 6px 12px;
      overflow-x: auto;
      border-bottom: 1px solid var(--vsc-border-subtle, var(--vsc-border));
      background: var(--vsc-neutral-overlay-subtle, transparent);
    }

    .cli-chip {
      flex: 0 0 auto;
      padding: 2px 8px;
      border: 1px solid var(--vsc-border-subtle, var(--vsc-border));
      border-radius: 4px;
      background: var(--vsc-input-background);
      color: var(--vsc-foreground);
      font-family: var(--vsc-font-family-mono, ui-monospace, monospace);
      font-size: 11px;
      cursor: pointer;
    }

    .cli-chip:hover {
      background: var(--vsc-list-hover-background);
      border-color: var(--vsc-accent-blue);
    }

    .cli-transcript {
      flex: 1 1 0%;
      min-height: 0;
      overflow: auto;
      padding: 12px;
      display: flex;
      flex-direction: column;
      gap: 12px;
      font-family: var(--vsc-font-family-mono, ui-monospace, monospace);
      font-size: 12px;
      line-height: 1.48;
      user-select: text;
    }

    .cli-banner {
      padding: 9px 11px;
      border: 1px solid var(--vsc-border-subtle, var(--vsc-border));
      border-radius: 6px;
      background: var(--vsc-neutral-overlay-subtle, transparent);
      font-family: var(--vsc-font-family, system-ui, sans-serif);
      font-size: 12px;
    }

    .cli-banner-sub {
      margin-top: 4px;
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
      font-size: 11px;
      line-height: 1.45;
    }

    .cli-banner code,
    .cli-banner kbd {
      font-family: var(--vsc-font-family-mono, ui-monospace, monospace);
      font-size: 11px;
      padding: 0 4px;
      border-radius: 3px;
      background: var(--vsc-input-background);
      border: 1px solid var(--vsc-border-subtle, var(--vsc-border));
    }

    .cli-entry {
      display: flex;
      flex-direction: column;
      gap: 4px;
    }

    .cli-entry-cmd {
      display: flex;
      align-items: baseline;
      gap: 6px;
      flex-wrap: wrap;
      font-size: 12px;
    }

    .cli-prompt-cwd {
      color: var(--vsc-accent-blue);
      font-weight: 600;
    }

    .cli-prompt-sigil {
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
    }

    .cli-prompt-text {
      font-weight: 600;
    }

    .cli-entry-meta {
      margin-left: auto;
      display: flex;
      align-items: center;
      gap: 8px;
      font-size: 11px;
    }

    .cli-exit-fail {
      color: var(--vsc-error);
      font-weight: 600;
    }

    .cli-duration {
      color: var(--vsc-description-foreground, var(--vsc-foreground-dim));
    }

    .cli-output {
      margin: 0;
      padding: 6px 10px;
      border-left: 2px solid var(--vsc-border-subtle, var(--vsc-border));
      white-space: pre-wrap;
      word-break: break-word;
      font-family: inherit;
      font-size: 12px;
    }

    .cli-output--err {
      border-left-color: var(--vsc-error);
      color: var(--vsc-error);
    }

    .cli-running {
      color: var(--vsc-accent-blue);
      font-size: 12px;
    }

    .cli-input-bar {
      display: flex;
      align-items: center;
      gap: 8px;
      padding: 8px 12px;
      border-top: 1px solid var(--vsc-border-subtle, var(--vsc-border));
      background: var(--vsc-toolbar-background, var(--vsc-panel-background));
    }

    .cli-prompt-label {
      flex: 0 0 auto;
      max-width: 200px;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      color: var(--vsc-accent-blue);
      font-family: var(--vsc-font-family-mono, ui-monospace, monospace);
      font-size: 12px;
      font-weight: 600;
    }

    .cli-input {
      flex: 1 1 0%;
      min-width: 0;
      height: 30px;
      padding: 0 10px;
      border: 1px solid var(--vsc-border);
      border-radius: 4px;
      background: var(--vsc-input-background);
      color: var(--vsc-foreground);
      font-family: var(--vsc-font-family-mono, ui-monospace, monospace);
      font-size: 12px;
      outline: none;
    }

    .cli-input:focus {
      border-color: var(--vsc-focus-border, var(--vsc-accent-blue));
    }

    .ansi-bold { font-weight: 700; }
    .ansi-dim { opacity: 0.72; }
    .ansi-red { color: var(--vsc-error); }
    .ansi-green { color: var(--vsc-success); }
    .ansi-yellow { color: var(--vsc-accent-yellow); }
    .ansi-blue { color: var(--vsc-accent-blue); }
    .ansi-magenta { color: var(--vsc-accent-purple, var(--vsc-accent-blue)); }
    .ansi-cyan { color: var(--vsc-accent-blue); }
    .ansi-gray { color: var(--vsc-description-foreground, var(--vsc-foreground-dim)); }
  `],
})
export class CliSurfaceTabComponent implements OnChanges {
  @Input({ required: true }) ctx!: SurfaceContext;
  @Input() active = false;

  @ViewChild('cmdInput') private cmdInput?: ElementRef<HTMLInputElement>;
  @ViewChild('scrollArea') private scrollArea?: ElementRef<HTMLDivElement>;

  private readonly api = inject(ApiService);
  private readonly workspace = inject(WorkspaceService);
  private readonly feedback = inject(ActionFeedbackService);
  private readonly cdr = inject(ChangeDetectorRef);

  cwd = '/';
  commandInput = '';
  running = false;
  activeCommandLabel = '';
  transcript: CliTranscriptEntry[] = [];

  private history: string[] = [];
  private historyIndex = -1;
  private initializedLocation = '';
  private nextEntryId = 1;

  ngOnChanges(): void {
    const loc = this.ctx?.tab?.path ?? '';
    if (loc && loc !== this.initializedLocation) {
      this.initializedLocation = loc;
      this.cwd = this.extractCwd(loc) || this.peerDirectoryPath || '/';
      if (this.transcript.length === 0) {
        void this.runQuickCommand('doctor');
      }
    }
    if (this.active) {
      queueMicrotask(() => this.focusPrompt());
    }
  }

  get peerDirectoryPath(): string | null {
    const peerPane = this.ctx?.paneId === 'left' ? 'right' : 'left';
    const tabs = this.workspace.getTabs(peerPane);
    const activeTab = tabs.find((t) => t.isActive) ?? tabs[0];
    if (!activeTab) return null;
    if ((activeTab.surfaceId ?? activeTab.kind ?? 'dir') !== 'dir') return null;
    return activeTab.path || null;
  }

  syncCwdFromPeer(): void {
    const peer = this.peerDirectoryPath;
    if (!peer) return;
    this.cwd = peer;
    this.feedback.show('info', 'CLI directory synced', peer);
    this.cdr.markForCheck();
    this.focusPrompt();
  }

  compareWithPeer(): void {
    const peer = this.peerDirectoryPath;
    if (!peer) return;
    void this.runQuickCommand(`compare "${this.cwd}" "${peer}"`);
  }

  prefillCommand(prefix: string): void {
    this.commandInput = prefix;
    this.cdr.markForCheck();
    this.focusPrompt();
  }

  runQuickCommand(command: string): void {
    this.commandInput = command;
    void this.submitCommand();
  }

  onCommandInput(event: Event): void {
    this.commandInput = (event.target as HTMLInputElement).value;
    this.historyIndex = -1;
  }

  async submitCommand(): Promise<void> {
    const raw = this.commandInput.trim();
    if (!raw || this.running) return;

    this.commandInput = '';
    this.historyIndex = -1;
    if (this.history[0] !== raw) {
      this.history.unshift(raw);
      if (this.history.length > 100) this.history.length = 100;
    }

    const lowered = raw.toLowerCase();
    if (lowered === 'clear' || lowered === 'cls') {
      this.clearTranscript();
      return;
    }

    this.running = true;
    this.activeCommandLabel = raw;
    const execCwd = this.cwd;
    this.cdr.markForCheck();

    try {
      const result = await this.api.executeCliCommand(raw, execCwd, true);
      if (result.changedCwd && result.cwd) {
        this.cwd = result.cwd;
      }

      this.transcript = [
        ...this.transcript,
        {
          id: this.nextEntryId++,
          cwd: execCwd,
          command: raw,
          segments: parseAnsiText(result.stdout),
          stderrSegments: parseAnsiText(result.stderr),
          exitCode: result.exitCode,
          durationMs: result.durationMs,
        },
      ];

      const head = raw.replace(/^(aether|reassembly)\s+/i, '').split(/\s+/)[0]?.toLowerCase() ?? '';
      if (result.ok && MUTATING_COMMANDS.has(head)) {
        this.feedback.show('info', `CLI completed: ${head}`, this.cwd);
      }
    } catch (err) {
      this.transcript = [
        ...this.transcript,
        {
          id: this.nextEntryId++,
          cwd: execCwd,
          command: raw,
          segments: [],
          stderrSegments: parseAnsiText(err instanceof Error ? err.message : String(err)),
          exitCode: 1,
          durationMs: 0,
        },
      ];
    } finally {
      this.running = false;
      this.activeCommandLabel = '';
      this.cdr.markForCheck();
      queueMicrotask(() => {
        const el = this.scrollArea?.nativeElement;
        if (el) el.scrollTop = el.scrollHeight;
        this.focusPrompt();
      });
    }
  }

  historyUp(event: Event): void {
    event.preventDefault();
    event.stopPropagation();
    if (this.history.length === 0) return;
    if (this.historyIndex < this.history.length - 1) {
      this.historyIndex++;
      this.commandInput = this.history[this.historyIndex];
      this.cdr.markForCheck();
    }
  }

  historyDown(event: Event): void {
    event.preventDefault();
    event.stopPropagation();
    if (this.historyIndex > 0) {
      this.historyIndex--;
      this.commandInput = this.history[this.historyIndex];
    } else if (this.historyIndex === 0) {
      this.historyIndex = -1;
      this.commandInput = '';
    }
    this.cdr.markForCheck();
  }

  async completeInput(event: Event): Promise<void> {
    event.preventDefault();
    event.stopPropagation();
    const val = this.commandInput;
    const parts = val.split(/\s+/);

    if (parts.length <= 1 && !val.endsWith(' ')) {
      const prefix = parts[0] || '';
      const hit = KNOWN_COMMANDS.find((c) => c.startsWith(prefix.toLowerCase()));
      if (hit) {
        this.commandInput = `${hit} `;
        this.cdr.markForCheck();
      }
      return;
    }

    const partial = val.endsWith(' ') ? '' : (parts[parts.length - 1] ?? '');
    try {
      const listing = await this.api.listDirectory(this.cwd, true);
      const matches = (listing?.entries ?? [])
        .map((e) => e.name)
        .filter((name) => !partial || name.toLowerCase().startsWith(partial.toLowerCase()));
      if (matches.length === 1) {
        const replacement = matches[0].includes(' ') ? `"${matches[0]}"` : matches[0];
        if (val.endsWith(' ')) {
          this.commandInput = `${val}${replacement}`;
        } else {
          parts[parts.length - 1] = replacement;
          this.commandInput = parts.join(' ');
        }
        this.cdr.markForCheck();
      }
    } catch {
      // Ignore autocomplete errors
    }
  }

  clearTranscript(event?: Event): void {
    event?.preventDefault();
    event?.stopPropagation();
    this.transcript = [];
    this.cdr.markForCheck();
    this.focusPrompt();
  }

  focusPrompt(): void {
    this.cmdInput?.nativeElement?.focus({ preventScroll: true });
  }

  shortCwd(full: string): string {
    if (!full || full === '/') return '/';
    const segments = full.split('/').filter(Boolean);
    if (segments.length <= 2) return full;
    return `…/${segments.slice(-2).join('/')}`;
  }

  segmentClass(seg: AnsiSegment): string {
    const classes: string[] = [];
    if (seg.bold) classes.push('ansi-bold');
    if (seg.dim) classes.push('ansi-dim');
    if (seg.color) classes.push(`ansi-${seg.color}`);
    return classes.join(' ');
  }

  private extractCwd(location: string): string {
    try {
      const parsed = new URL(location);
      return parsed.searchParams.get('cwd') || '';
    } catch {
      return '';
    }
  }
}
