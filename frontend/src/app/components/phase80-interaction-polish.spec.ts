import { readFileSync } from 'node:fs';

describe('Phase 80 Interaction Polish', () => {
  const pane = readFileSync(
    new URL('./directory-pane.component.ts', import.meta.url),
    'utf8',
  );
  const address = readFileSync(
    new URL('./address-bar.component.ts', import.meta.url),
    'utf8',
  );
  const menu = readFileSync(
    new URL('./context-menu.component.ts', import.meta.url),
    'utf8',
  );
  const viewer = readFileSync(
    new URL('../viewers/viewer-host-surface.component.ts', import.meta.url),
    'utf8',
  );

  it('hands keyboard ownership to a clicked selection', () => {
    expect(pane).toContain('A pointer selection immediately hands keyboard ownership');
    expect(pane).toContain('this.focusContent()');
  });

  it('keeps the whole pane free of the unwanted focus border', () => {
    expect(pane).not.toContain('.pane.active-pane');
    expect(pane).toContain('.list:focus-visible,.tree-view:focus-visible,.pane:focus-visible');
    expect(pane).toContain('outline:none');
  });

  it('enters address editing from empty address-bar space on one click', () => {
    expect(address).toContain('(click)="onDisplayPathClick($event)"');
    expect(address).toContain('this.cdr.detectChanges()');
    expect(address).toContain('this.focusAndSelect()');
    expect(address).not.toContain('class="edit-path"');
  });

  it('supports rename-next and rename-previous with Tab', () => {
    expect(pane).toContain("if (event.key === 'Tab')");
    expect(pane).toContain('const delta = event.shiftKey ? -1 : 1');
    expect(pane).toContain('this.beginInlineRename(next)');
  });

  it('spring-opens a directory during an accepted drag hover', () => {
    expect(pane).toContain('scheduleDragSpringOpen(entry)');
    expect(pane).toContain('}, 700)');
    expect(pane).toContain('cancelDragSpringOpen()');
  });

  it('makes context menus keyboard navigable and viewport fitted', () => {
    expect(menu).toContain('fitToViewport()');
    expect(menu).toContain("event.key === 'ArrowDown'");
    expect(menu).toContain("event.key === 'ArrowUp'");
    expect(menu).toContain("event.key === 'Escape'");
    expect(menu).toContain('startsWith(this.typeahead)');
  });

  it('does not force a context-menu vertical scrollbar', () => {
    expect(menu).toContain('overflow: visible');
    expect(menu).toContain('columns: 230px auto');
  });

  it('retains viewer content while a new viewer resolves', () => {
    expect(viewer).toContain('viewer-stage');
    expect(viewer).toContain('resolving = false');
    expect(viewer).toContain('Loading preview…');
    expect(viewer).toContain('prefers-reduced-motion');
  });
});
