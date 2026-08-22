import { readFileSync } from 'node:fs';

const edit = readFileSync(new URL('./address-bar-edit.component.ts', import.meta.url), 'utf8');
const catalog = readFileSync(new URL('../services/dialog-input-catalog.ts', import.meta.url), 'utf8');

describe('address bar edit shortcut model', () => {
  it('leaves Tab and Shift+Tab to native focus traversal', () => {
    expect(catalog).not.toContain('widget.addressBar.edit.tab.next');
    expect(catalog).not.toContain('widget.addressBar.edit.tab.prev');
    expect(catalog).not.toContain("'widget.addressBar.edit.nextSuggestion'");
    expect(catalog).not.toContain("'widget.addressBar.edit.prevSuggestion'");
  });

  it('uses Ctrl+Space as the explicit autocomplete shortcut', () => {
    expect(catalog).toContain(
      "keyBinding('widget.addressBar.edit.complete.ctrlSpace', 'widget.addressBar.edit', 'widget.addressBar.edit.showSuggestions', 'space', 'Show address suggestions', ['ctrl'], 'typing')",
    );
    expect(edit).toContain(
      "this.inputEngine.registerCommand('widget.addressBar.edit.showSuggestions'",
    );
  });

  it('bridges input-owned keydown events into the canonical input engine', () => {
    expect(edit).toContain('this.inputEngine.dispatchKeyboardEvent(e');
    expect(edit).toContain('context: { inInput: true }');
    expect(edit).toContain('if (handled)');
  });

  it('keeps arrows and Enter in the canonical address-bar binding catalog', () => {
    expect(catalog).toContain('widget.addressBar.edit.commit.enter');
    expect(catalog).toContain('widget.addressBar.edit.prev.arrowUp');
    expect(catalog).toContain('widget.addressBar.edit.next.arrowDown');
  });
  it('marks address-bar edit bindings as valid while typing', () => {
    expect(catalog).toContain("'widget.addressBar.edit.commit', 'enter', 'Commit address bar path', undefined, 'typing'");
    expect(catalog).toContain("'widget.addressBar.edit.cancel', 'escape', 'Cancel address bar edit', undefined, 'typing'");
    expect(catalog).toContain("'widget.addressBar.edit.prevItem', 'arrowup', 'Select previous address suggestion', undefined, 'typing'");
    expect(catalog).toContain("'widget.addressBar.edit.nextItem', 'arrowdown', 'Select next address suggestion', undefined, 'typing'");
    expect(catalog).toContain("'widget.addressBar.edit.showSuggestions', 'space', 'Show address suggestions', ['ctrl'], 'typing'");
  });

});
