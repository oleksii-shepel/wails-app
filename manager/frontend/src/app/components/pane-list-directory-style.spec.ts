import { readFileSync } from 'node:fs';

const source = readFileSync(new URL('./pane-list.component.ts', import.meta.url), 'utf8');

describe('PaneList directory-style layout', () => {
  it('uses the same horizontal overflow surface as directory list view', () => {
    expect(source).toContain('overflow-x: auto;');
    expect(source).toContain('grid-template-columns: minmax(100%, max-content);');
  });

  it('keeps list rows stretched on one shared track with the standard side inset', () => {
    expect(source).toContain('grid-column: 1;');
    expect(source).toContain('justify-self: stretch;');
    expect(source).toContain('margin-left: 4px;');
    expect(source).toContain('margin-right: 4px;');
  });
});
