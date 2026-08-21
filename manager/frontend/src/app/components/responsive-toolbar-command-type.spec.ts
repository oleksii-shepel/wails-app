import { readFileSync } from 'node:fs';

const source = readFileSync(
  new URL('./responsive-toolbar.component.ts', import.meta.url),
  'utf8',
);

describe('ResponsiveToolbarCommand type contract', () => {
  it('exports the extended toolbar command type', () => {
    expect(source).toContain('export type ResponsiveToolbarCommand = PresentedCommand & {');
    expect(source).toContain('readonly active?: boolean;');
  });

  it('uses the extended type for every template-visible command collection', () => {
    expect(source).toContain('@Input() actions: readonly ResponsiveToolbarCommand[] = [];');
    expect(source).toContain('visibleActions: readonly ResponsiveToolbarCommand[] = [];');
    expect(source).toContain('overflowActions: readonly ResponsiveToolbarCommand[] = [];');
    expect(source).toContain('invoke(action: ResponsiveToolbarCommand): void');
    expect(source).toContain('trackAction(_index: number, action: ResponsiveToolbarCommand)');
  });
});
