import { readFileSync } from 'node:fs';

describe('Registry hidden-file status', () => {
  const pane = readFileSync(
    new URL('./directory-pane.component.ts', import.meta.url),
    'utf8',
  );

  it('does not expose filesystem hidden-file state on Registry surfaces', () => {
    expect(pane).toContain(
      '*ngIf="showHidden && !isRegistrySurface" class="hidden-state"',
    );
  });

  it('keeps filesystem hidden status available outside Registry', () => {
    expect(pane).toContain('hidden visible');
  });
});
