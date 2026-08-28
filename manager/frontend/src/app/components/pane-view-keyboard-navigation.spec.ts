import { readFileSync } from 'node:fs';

const directorySource = readFileSync(
  new URL('./directory-pane.component.ts', import.meta.url),
  'utf8',
);
const dualPaneSource = readFileSync(
  new URL('./dual-pane.component.ts', import.meta.url),
  'utf8',
);

describe('Pane keyboard navigation', () => {
  it('supports arrow, Home, and End navigation across view controls', () => {
    expect(directorySource).toContain('(keydown)=\"onViewSwitcherKeydown($event)\"');
    expect(directorySource).toContain("['ArrowLeft', 'ArrowRight', 'Home', 'End']");
    expect(directorySource).toContain("target.tagName === 'SELECT'");
  });

  it('uses Tab and Shift+Tab for pane switching', () => {
    expect(dualPaneSource).toContain("id:'reassembly.manager.switchPane.tab'");
    expect(dualPaneSource).toContain("id:'reassembly.manager.switchPane.shiftTab'");
    expect(dualPaneSource).toContain("{key:'tab', modifiers:['shift']}");
  });

  it('does not reserve Ctrl+Alt+D or Ctrl+Alt+T for view switching', () => {
    expect(directorySource).not.toContain('Ctrl+Alt+D');
    expect(directorySource).not.toContain('Ctrl+Alt+T');
    expect(directorySource).not.toContain('onPaneViewShortcut');
  });
});
