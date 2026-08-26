import { readFileSync } from 'node:fs';

const component = readFileSync(
  new URL('./quick-access.component.ts', import.meta.url),
  'utf8',
);

describe('Quick Access icon layout', () => {
  it('normalizes semantic system-folder icon names before rendering', () => {
    expect(component).toContain('SPECIAL_LOCATION_ICONS');
    expect(component).toContain("home: '\\u{1F3E0}'");
    expect(component).toContain('icon: this.iconFor(folder.icon, DEFAULT_LOCATION_ICON)');
    expect(component).not.toContain("icon: folder.icon || '");
  });

  it('keeps shortcut badges and glyphs inside a fixed leading column', () => {
    expect(component).toContain('<span class="glyph">{{ item.icon }}</span>');
    expect(component).toContain('grid-template-columns:40px minmax(0,1fr) max-content');
    expect(component).toContain('.icon{min-width:0;overflow:hidden;');
    expect(component).toContain('.glyph{min-width:0;overflow:hidden;white-space:nowrap;');
  });
});
