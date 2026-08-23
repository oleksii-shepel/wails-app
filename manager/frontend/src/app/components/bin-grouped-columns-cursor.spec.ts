import { readFileSync } from 'node:fs';

const source = readFileSync(new URL('./bin-pane.component.ts', import.meta.url), 'utf8');

describe('Bin grouped columns and cursor parity', () => {
  it('uses a real Location column in grouped mode', () => {
    expect(source).toContain(
      '<span></span><span>Name</span><span>Location</span><span>Deleted</span><span>Size</span><span>Revision</span>',
    );
    expect(source).toContain('class="kind bin-group-location"');
  });

  it('does not render filename and location as inline flex fragments', () => {
    expect(source).not.toContain('bin-group-filename');
    expect(source).not.toContain('bin-group-directory');
    expect(source).not.toContain('gap: 12px;');
  });

  it('uses normal Directory cursor styling for group headers', () => {
    expect(source).not.toContain('bin-group-cursor');
    expect(source).toContain('[class.cursor]="entry.path === cursorPath"');
    expect(source).toContain('.list.view-details.bin-grouped .entry.bin-group-header {');
    expect(source).toContain('margin-left: 0;');
  });

  it('uses foreground palette for the disclosure mark', () => {
    expect(source).toContain('.bin-group-header .bin-disclosure {');
    expect(source).toContain('color: var(--vsc-foreground);');
    expect(source).toContain('background: transparent;');
  });
});
