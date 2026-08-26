import { readFileSync } from 'node:fs';

const read = (name: string) => readFileSync(new URL(`./${name}`, import.meta.url), 'utf8');

const animated = [
  'operation-journal.component.ts',
  'global-search.component.ts',
  'operation-center.component.ts',
  'selection-basket.component.ts',
  'path-history.component.ts',
];

describe('main-menu dialog standardization', () => {
  for (const file of animated) {
    it(`${file} retains the standard surface through exit animation`, () => {
      const source = read(file);
      expect(source).toContain('dialog-group');
      expect(source).toContain('dialog-surface');
      expect(source).toContain('dialog-group--closing');
      expect(source).toContain('onAnimationEnd');
    });
  }

  it('uses the shared close button in Operation Center', () => {
    const source = read('operation-center.component.ts');
    expect(source).toContain('class="dialog-close-btn"');
    expect(source).not.toContain('class="icon-button"');
  });

  it('uses Global Search as the standard dialog title', () => {
    const source = read('global-search.component.ts');
    expect(source).toContain('<strong>Global Search</strong>');
  });
});
