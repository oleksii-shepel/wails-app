import { readFileSync } from 'node:fs';

describe('Registry sort controls', () => {
  const pane = readFileSync(
    new URL('./directory-pane.component.ts', import.meta.url),
    'utf8',
  );

  it('does not render filesystem sort controls on Registry surfaces', () => {
    expect(pane).toContain(
      '*ngIf="!isRegistrySurface" class="sort-controls"',
    );
  });

  it('ignores generic sort mutations while Registry is active', () => {
    const guards = pane.match(/if \(this\.isRegistrySurface\) return;/g) ?? [];
    expect(guards.length).toBeGreaterThanOrEqual(3);
  });
});
