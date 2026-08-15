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

@Component({
  selector: 'app-shortcut-debug-overlay',
  standalone: true,
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <aside class="overlay" aria-label="Shortcut debugger">
      <header>
        <strong>Shortcut debugger</strong>
        <span>visible during reassembly</span>
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

      <div class="bindings">
        <div><kbd>Up</kbd><kbd>Down</kbd><span>cursor</span></div>
        <div><kbd>Enter</kbd><span>open folder</span></div>
        <div><kbd>Backspace</kbd><span>parent</span></div>
        <div><kbd>Alt</kbd><span>+</span><kbd>Left/Right</kbd><span>history</span></div>
        <div><kbd>Ctrl</kbd><span>+</span><kbd>L</kbd><span>location</span></div>
        <div><kbd>Ctrl</kbd><span>+</span><kbd>A</kbd><span>select all</span></div>
        <div><kbd>Ctrl</kbd><span>+</span><kbd>R</kbd><span>refresh</span></div>
        <div><kbd>F2</kbd><span>rename</span></div>
        <div><kbd>F7</kbd><span>new folder</span></div>
        <div><kbd>Delete/F8</kbd><span>delete</span></div>
        <div><kbd>Ctrl+C/X/V</kbd><span>clipboard</span></div>
        <div><kbd>Tab</kbd><span>switch pane</span></div>
        <div><kbd>F5</kbd><span>copy to other pane</span></div>
        <div><kbd>F6</kbd><span>move to other pane</span></div>
        <div><kbd>Ctrl+T</kbd><span>new tab</span></div>
        <div><kbd>Ctrl+W</kbd><span>close tab</span></div>
        <div><kbd>Ctrl+Tab</kbd><span>next tab</span></div>
        <div><kbd>Enter</kbd><span>folder or viewer</span></div>
        <div><kbd>Left/Right</kbd><span>viewer previous/next</span></div>
        <div><kbd>Ctrl++/-</kbd><span>viewer zoom</span></div>
        <div><kbd>Ctrl+0/1</kbd><span>fit / actual size</span></div>
        <div><kbd>Right click</kbd><span>command context menu</span></div>
        <div><kbd>Drag</kbd><span>copy · Shift+drop move</span></div>
        <div><span>Live operation stream</span><span>bottom-left</span></div>
        <div><span>Directory tabs</span><span>workspace persisted</span></div>
        <div><kbd>Drag divider</kbd><span>resize · double-click reset</span></div>
        <div><kbd>Ctrl+F</kbd><span>filter folder</span></div>
        <div><kbd>F3 / Shift+F3</kbd><span>next / previous match</span></div>
        <div><kbd>Ctrl+Shift+H</kbd><span>show / hide hidden</span></div>
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
    }

    .overlay {
      box-sizing: border-box;
      width: min(430px, calc(100vw - 20px));
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
  `],
})
export class ShortcutDebugOverlayComponent {
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

  constructor(public readonly input: InputEngineService) {}

  protected pendingChordStepCount(value: unknown): number {
    if (!value || typeof value !== 'object') {
      return 0;
    }

    const chord = value as Partial<PendingChordLike>;
    return Array.isArray(chord.steps) ? chord.steps.length : 0;
  }
}
