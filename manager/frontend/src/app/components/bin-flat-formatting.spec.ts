import { readFileSync } from 'node:fs';

const source = readFileSync(new URL('./bin-pane.component.ts', import.meta.url), 'utf8');

describe('Bin flat formatting', () => {
  it('uses numeric revisions only', () => {
    expect(source).toContain('label: String(group.length - index)');
    expect(source).not.toContain("'Latest'");
    expect(source).not.toContain('`r${group.length - index}`');
    expect(source).not.toContain('`v${group.length - index}`');
  });

  it('uses the flat Details schema', () => {
    expect(source).toContain('<span>Name</span><span>Location</span><span>Deleted</span><span>Size</span><span>Revision</span>');
  });

  it('uses Directory metadata classes for flat cells', () => {
    expect(source).toContain('class="kind bin-location"');
    expect(source).toContain('class="modified bin-deleted"');
    expect(source).toContain('class="size bin-size"');
    expect(source).toContain('class="kind bin-revision"');
  });
});
