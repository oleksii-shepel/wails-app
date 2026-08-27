import { readFileSync } from 'node:fs';

describe('inactive pane entry activation ordering', () => {
  const paneFiles = [
    'directory-pane.component.ts',
    'bin-pane.component.ts',
    'registry-pane.component.ts',
    'autorun-pane.component.ts',
  ];

  it('does not activate DirectoryPane-derived surfaces on mousedown', () => {
    for (const name of paneFiles) {
      const source = readFileSync(new URL(`./${name}`, import.meta.url), 'utf8');
      expect(source).withContext(name)
        .not.toContain('(mousedown)="activateRequested.emit()"');
      expect(source).withContext(name)
        .toContain('(focusin)="activateRequested.emit()"');
    }
  });

  it('activates dual-pane slots after child click handlers, not on pointerdown', () => {
    const source = readFileSync(new URL('./dual-pane.component.ts', import.meta.url), 'utf8');
    expect(source).toContain(`(click)="activate('left')"`);
    expect(source).toContain(`(click)="activate('right')"`);
    expect(source).not.toContain(`(pointerdown)="activate('left')"`);
    expect(source).not.toContain(`(pointerdown)="activate('right')"`);
  });
});
