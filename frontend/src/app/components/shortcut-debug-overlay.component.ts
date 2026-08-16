import {
  ChangeDetectionStrategy,
  Component,
  Input,
} from '@angular/core';
import { CommonModule } from '@angular/common';
import { InputEngineService } from '../services/input-engine.service';

interface PendingChordLike {
  steps: unknown[];
}

export interface ShortcutHelpItem {
  id: string;
  shortcut: string;
  description: string;
  category: string;
}

@Component({
  selector: 'app-shortcut-debug-overlay',
  standalone: true,
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <aside
      *ngIf="visible"
      class="overlay"
      role="dialog"
      aria-modal="false"
      aria-label="Keyboard shortcuts">
      <header>
        <strong>Keyboard shortcuts</strong>
        <span>F1 to close</span>
      </header>

      <div class="state">
        <span class="label">pane</span>
        <span class="value">{{ activePane }}</span>

        <span class="label">path</span>
        <span class="value mono">{{ path || '-' }}</span>

        <span class="label">other</span>
        <span class="value mono">{{ otherPath || '-' }}</span>

        <span class="label">cursor</span>
        <span class="value">{{ cursorName || '-' }}</span>

        <span class="label">selected</span>
        <span class="value">{{ selectedCount }}</span>

        <span class="label">action</span>
        <span class="value">{{ lastAction || '-' }}</span>

        <span class="label">transfer</span>
        <span class="value">{{ lastTransfer || '-' }}</span>

        <span class="label">drop</span>
        <span class="value">{{ lastDrop || '-' }}</span>

        <span class="label">tabs</span>
        <span class="value">{{ tabCount }}</span>

        <span class="label">surface</span>
        <span class="value">{{ surfaceKind }}</span>

        <span class="label">split</span>
        <span class="value">{{ splitPercent | number:'1.0-0' }} / {{ 100 - splitPercent | number:'1.0-0' }}</span>

        <span class="label">hidden</span>
        <span class="value">{{ showHidden ? 'visible' : 'hidden' }}</span>
      </div>

      <div class="pending" *ngIf="input.pendingChord$ | async as chord">
        chord pending · {{ pendingChordStepCount(chord) }} step{{ pendingChordStepCount(chord) === 1 ? '' : 's' }}
      </div>

      <div class="binding-toolbar">
        <strong>Live bindings</strong>
        <span>{{ bindings.length }} registered manager shortcuts</span>
      </div>

      <div class="bindings live-bindings">
        <div *ngFor="let binding of bindings; trackBy: trackBinding">
          <kbd>{{ binding.shortcut }}</kbd>
          <span>{{ binding.description }}</span>
          <small>{{ binding.category }}</small>
        </div>
      </div>

      <div class="binding-notes">
        <strong>Context notes</strong>
        <span>F3 / Shift+F3 navigate inline-filter matches while the filter owns focus.</span>
        <span>Viewer Left/Right and zoom keys are enabled only when the active viewer implements them.</span>
        <span>Overlay/dialog contexts override manager bindings while they are active.</span>
        <span>Filtered selections stay selected even while hidden; F3 moves the match cursor without collapsing a multi-selection.</span>
        <span>Workspace switcher: 1–9 activate · F2 rename · Delete remove · Enter activate.</span>
        <span>Global Search: Alt+R recursive · Alt+S sort · Alt+↑/↓ recent queries · Ctrl+Enter containing folder.</span>
        <span>Command Palette, Quick Access, and History use the same weighted discovery matcher for abbreviations and multi-token queries.</span>
        <span>Navigation overlays: Ctrl+Q Quick Access · Ctrl+H Location History · Ctrl+B Pinned Locations.</span>
        <span>Accessibility: modal lists expose selected state and item position; Tools uses tab semantics; operation failures are announced.</span>
        <span>Pinned tabs survive Close Other / Close Right; viewer tabs participate in move, reorder, pin and restore just like directory tabs.</span>
        <span>Embedded browser panes may forward function/modifier keys through their host bridge; the file-manager shell itself does not install global keyboard listeners.</span>
      </div>

      <footer>
        Keyboard ownership: <strong>InputEngineService</strong>
      </footer>
    </aside>
  `,
  styles: [`
    :host {
      position: fixed;
      top: 10px;
      right: 10px;
      z-index: 10000;
      pointer-events: none;
      max-height: calc(100vh - 20px);
    }

    .overlay {
      box-sizing: border-box;
      width: min(520px, calc(100vw - 20px));
      max-height: calc(100vh - 20px);
      overflow: auto;
      overscroll-behavior: contain;
      padding: 10px 12px;
      border: 1px solid color-mix(
        in srgb,
        var(--vsc-accent-blue, #007acc) 58%,
        var(--vsc-border, #3c3c3c)
      );
      border-radius: 7px;
      background: color-mix(
        in srgb,
        var(--vsc-editor-background, #1e1e1e) 94%,
        transparent
      );
      color: var(--vsc-foreground, #cccccc);
      box-shadow: 0 8px 28px rgba(0, 0, 0, .35);
      backdrop-filter: blur(8px);
      font: 11px/1.4 var(--vsc-font-family, system-ui, sans-serif);
    }

    header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      gap: 12px;
      margin-bottom: 8px;
    }

    header span,
    footer,
    .label {
      color: var(--vsc-description-foreground, #969696);
    }

    .state {
      display: grid;
      grid-template-columns: 54px minmax(0, 1fr);
      gap: 3px 8px;
      padding: 7px 0;
      border-top: 1px solid var(--vsc-border-subtle, #2b2b2b);
      border-bottom: 1px solid var(--vsc-border-subtle, #2b2b2b);
    }

    .value {
      min-width: 0;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .mono {
      font-family: var(--vsc-font-family-mono, ui-monospace, monospace);
    }

    .bindings {
      display: grid;
      gap: 5px;
      margin-top: 8px;
    }

    .bindings > div {
      display: flex;
      align-items: center;
      gap: 4px;
    }

    kbd {
      min-width: 18px;
      padding: 1px 5px;
      border: 1px solid var(--vsc-border, #3c3c3c);
      border-bottom-width: 2px;
      border-radius: 4px;
      background: var(--vsc-input-background, #181818);
      color: var(--vsc-foreground-bright, #ffffff);
      text-align: center;
      font: 10px/1.5 var(--vsc-font-family-mono, ui-monospace, monospace);
    }

    .binding-toolbar {
      display: flex;
      justify-content: space-between;
      gap: 8px;
      margin-top: 8px;
      color: var(--vsc-description-foreground, #969696);
    }

    .binding-toolbar strong {
      color: var(--vsc-foreground, #cccccc);
    }

    .live-bindings > div {
      grid-template-columns: minmax(110px, auto) minmax(0, 1fr) auto;
    }

    .live-bindings small {
      color: var(--vsc-description-foreground, #969696);
      white-space: nowrap;
    }

    .binding-notes {
      display: grid;
      gap: 4px;
      margin-top: 8px;
      padding-top: 7px;
      border-top: 1px solid var(--vsc-border-subtle, #2b2b2b);
      color: var(--vsc-description-foreground, #969696);
      font-size: 10px;
    }

    .binding-notes strong {
      color: var(--vsc-foreground, #cccccc);
    }

    .pending {
      margin-top: 7px;
      color: var(--vsc-accent-blue, #4aa3ff);
    }

    footer {
      margin-top: 8px;
      padding-top: 7px;
      border-top: 1px solid var(--vsc-border-subtle, #2b2b2b);
    }

    footer strong {
      color: var(--vsc-foreground, #cccccc);
    }
    @media (max-width:768px){
      :host{top:6px;right:6px;left:6px;max-height:calc(100vh - 12px)}
      .overlay{width:100%;max-height:calc(100vh - 12px)}
      .bindings>div{grid-template-columns:minmax(110px,.8fr) minmax(0,1fr)}
      .state{grid-template-columns:48px minmax(0,1fr)}
    }
  `],
})
export class ShortcutDebugOverlayComponent {
  @Input() visible = false;
  @Input() path = '';
  @Input() cursorName: string | null = null;
  @Input() selectedCount = 0;
  @Input() lastAction = '';
  @Input() activePane: 'left' | 'right' = 'left';
  @Input() otherPath = '';
  @Input() lastTransfer = '';
  @Input() lastDrop = '';
  @Input() tabCount = 0;
  @Input() surfaceKind = 'dir';
  @Input() splitPercent = 50;
  @Input() showHidden = false;
  @Input() bindings: readonly ShortcutHelpItem[] = [];

  constructor(public readonly input: InputEngineService) {}

  trackBinding(_index: number, binding: ShortcutHelpItem): string {
    return binding.id;
  }

  protected pendingChordStepCount(value: unknown): number {
    if (!value || typeof value !== 'object') {
      return 0;
    }

    const chord = value as Partial<PendingChordLike>;
    return Array.isArray(chord.steps) ? chord.steps.length : 0;
  }
}
