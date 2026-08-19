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

  it('keeps the address edit action visible outside breadcrumb scrolling', () => {
    expect(address).toContain('grid-template-columns: 24px minmax(0, 1fr) 24px;');
    expect(address).toContain('class="edit-path"');
    expect(address).toContain('aria-label="Edit location"');
  });

  it('keeps long breadcrumb paths horizontally scrollable with hidden scrollbars', () => {
    expect(address).toContain('#breadcrumbs');
    expect(address).toContain('(wheel)="onBreadcrumbWheel($event)"');
    expect(address).toContain('overflow-x: auto;');
    expect(address).toContain('scrollbar-width: none;');
    expect(address).toContain('.breadcrumbs::-webkit-scrollbar');
    expect(address).toContain('el.scrollLeft = el.scrollWidth');
  });

  it('enters address editing from empty address-bar space on one click', () => {
    expect(address).toContain('(click)="onDisplayPathClick($event)"');
    expect(address).toContain('this.cdr.detectChanges()');
    expect(address).toContain('this.focusAndSelect()');
    expect(address).toContain('class="edit-path"');
    expect(address).toContain('(click)="beginEdit(); $event.stopPropagation()"');
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

  it('keeps context menus in one bounded column', () => {
    expect(menu).toContain('overflow-y: auto');
    expect(menu).toContain('max-height: calc(100vh - 16px)');
    expect(menu).toContain('overscroll-behavior: contain');
    expect(menu).not.toContain('columns: 230px auto');
    expect(menu).not.toContain('column-fill: auto');
  });

  it('retains viewer content while a new viewer resolves', () => {
    expect(viewer).toContain('viewer-stage');
    expect(viewer).toContain('resolving = false');
    expect(viewer).toContain('Loading preview…');
    expect(viewer).toContain('prefers-reduced-motion');
  });
});