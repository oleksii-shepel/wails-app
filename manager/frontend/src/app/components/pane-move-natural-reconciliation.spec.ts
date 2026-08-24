import { readFileSync } from 'node:fs';

const component = (name: string) =>
  readFileSync(new URL(`./${name}`, import.meta.url), 'utf8');
const service = (name: string) =>
  readFileSync(new URL(`../services/${name}`, import.meta.url), 'utf8');

describe('natural pane move reconciliation', () => {
  it('publishes batch move/copy mutations instead of relying on command refreshes', () => {
    const api = service('api.service.ts');
    expect(api).toContain("kind: 'move'");
    expect(api).toContain("origin: 'batch-move'");
    expect(api).toContain("kind: 'copy'");
    expect(api).toContain("origin: 'batch-copy'");
  });

  it('does not eagerly reload both panes after ordinary F5/F6 transfer', () => {
    const dual = component('dual-pane.component.ts');
    const start = dual.indexOf("private async transferToOther(operation: 'copy' | 'move')");
    const end = dual.indexOf('private isRecycleBinPath', start);
    const method = dual.slice(start, end);
    expect(method).not.toContain('await destination.navigation?.refresh();');
    expect(method).not.toContain('await source.navigation?.refresh();');
  });

  it('does not eagerly reload a filesystem paste destination', () => {
    const clipboard = component('pane-clipboard.controller.ts');
    const start = clipboard.indexOf('async paste(host: PaneClipboardHost)');
    const end = clipboard.indexOf('private cloneArchiveFrame', start);
    const method = clipboard.slice(start, end);
    expect(method).toContain('host.focusContent?.();');
    expect(method).not.toContain('await host.refresh();\n      host.focusContent?.();');
  });

  it('recovers a moved-away cursor at the same row index', () => {
    const pane = component('directory-pane.component.ts');
    expect(pane).toContain('cursorIndexBeforeRefresh');
    expect(pane).toContain('reconcileSelectionAfterRefresh(');
    expect(pane).toContain('Math.min(previousCursorIndex, selectable.length - 1)');
  });

  it('preserves surviving multi-selection across watcher reconciliation', () => {
    const pane = component('directory-pane.component.ts');
    expect(pane).toContain('selectionBeforeRefresh');
    expect(pane).toContain('survivingSelection');
  });
});
