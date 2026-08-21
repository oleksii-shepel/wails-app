import { readFileSync } from 'node:fs';

const dual = readFileSync(
  new URL('./dual-pane.component.ts', import.meta.url),
  'utf8',
);
const host = readFileSync(
  new URL('./pane-host.component.ts', import.meta.url),
  'utf8',
);
const directory = readFileSync(
  new URL('./directory-pane.component.ts', import.meta.url),
  'utf8',
);
const registry = readFileSync(
  new URL('./registry-pane.component.ts', import.meta.url),
  'utf8',
);

describe('pane context-menu event chain', () => {
  it('wires Directory/Registry entry menus from both PaneHost instances into DualPane', () => {
    expect(dual).toContain(
      `(entryContextMenu)="openEntryMenu('left', $event.event, $event.entry)"`,
    );
    expect(dual).toContain(
      `(entryContextMenu)="openEntryMenu('right', $event.event, $event.entry)"`,
    );
  });

  it('wires background menus from both PaneHost instances into DualPane', () => {
    expect(dual).toContain(
      `(backgroundContextMenu)="openBackgroundMenu('left', $event)"`,
    );
    expect(dual).toContain(
      `(backgroundContextMenu)="openBackgroundMenu('right', $event)"`,
    );
  });

  it('keeps PaneHost forwarding standard pane context-menu outputs', () => {
    expect(host).toContain(`(entryContextMenu)="entryContextMenu.emit($event)"`);
    expect(host).toContain(`(backgroundContextMenu)="backgroundContextMenu.emit($event)"`);
  });

  it('keeps both standard panes emitting entry and background context-menu events', () => {
    for (const source of [directory, registry]) {
      expect(source).toContain('this.entryContextMenu.emit({ event, entry });');
      expect(source).toContain('this.backgroundContextMenu.emit(event);');
    }
  });

  it('keeps the shared menu overlay mounted', () => {
    expect(dual).toContain('<app-context-menu');
    expect(dual).toContain('[state]="contextMenu"');
    expect(dual).toContain('(command)="dispatchContextCommand($event)"');
  });
});
