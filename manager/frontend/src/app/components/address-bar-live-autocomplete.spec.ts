import { readFileSync } from 'node:fs';

const address = readFileSync(new URL('./address-bar.component.ts', import.meta.url), 'utf8');

describe('live address bar autocomplete', () => {
  it('binds Ctrl+Space in the live editor context', () => {
    expect(address).toContain('complete.ctrlSpace');
    expect(address).toContain("key: 'space', modifiers: ['ctrl']");
    expect(address).toContain("complete: 'completeSuggestions'");
  });

  it('routes explicit completion through the resolved panel source', () => {
    expect(address).toContain('this.panelSources.resolve(context)');
    expect(address).toContain('await resolved.source.list(context, false)');
  });

  it('treats C:/ as the C drive root with an empty prefix', () => {
    expect(address).toContain("if (/^[a-z]:\\/$/i.test(raw))");
    expect(address).toContain("dirPath: root.endsWith('/') ? root : `${root}/`");
    expect(address).toContain("prefix: ''");
  });

  it('lets arrows populate suggestions when the dropdown is closed', () => {
    expect(address).toContain('await this.forceSourceSuggestions()');
  });

  it('bounds source results for large roots', () => {
    expect(address).toContain('if (suggestions.length >= 100) break;');
  });
});
