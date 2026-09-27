import { readFileSync } from 'node:fs';

const read = (name: string) => readFileSync(new URL(`./${name}`, import.meta.url), 'utf8');

const directorySource = read('directory-pane.component.ts');
const dualPaneSource = read('dual-pane.component.ts');
const gridPaneSources = [
  directorySource,
  read('autorun-pane.component.ts'),
  read('registry-pane.component.ts'),
  read('network-pane.component.ts'),
  read('environment-pane.component.ts'),
  read('bin-pane.component.ts'),
];

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

  it('uses two-dimensional cursor geometry in every grid-backed pane', () => {
    for (const source of gridPaneSources) {
      expect(source).toContain("if (this.viewMode === 'grid')");
      expect(source).toContain('this.moveGridCursorHorizontal(-1);');
      expect(source).toContain('this.moveGridCursorHorizontal(1);');
      expect(source).toContain("const cursorStep = this.viewMode === 'grid'");
      expect(source).toContain('delta * cursorStep');
    }
  });

  it('routes Left and Right to grid navigation as well as tree navigation', () => {
    expect(dualPaneSource).toContain("['tree', 'grid'].includes(this.activePane?.contextState.viewMode ?? '')");
    expect(dualPaneSource).toContain("description:'Move cursor left or collapse tree node'");
    expect(dualPaneSource).toContain("description:'Move cursor right or expand tree node'");
  });

  it('stops captured arrow keys so tree navigation is not applied twice', () => {
    expect(dualPaneSource).toContain("id:'reassembly.manager.cursorUp.arrowUp', commandId:COMMAND.cursorUp");
    expect(dualPaneSource).toContain("id:'reassembly.manager.cursorDown.arrowDown', commandId:COMMAND.cursorDown");
    expect(dualPaneSource).toContain("preventDefault:true, stopPropagation:true, trigger:{device:'keyboard', chord:{steps:[{key:'arrowup'}]}}");
    expect(dualPaneSource).toContain("preventDefault:true, stopPropagation:true, trigger:{device:'keyboard', chord:{steps:[{key:'arrowdown'}]}}");
    expect(dualPaneSource).toContain("preventDefault:true, stopPropagation:true, trigger:{device:'keyboard', chord:{steps:[{key:'arrowleft'}]}}");
    expect(dualPaneSource).toContain("preventDefault:true, stopPropagation:true, trigger:{device:'keyboard', chord:{steps:[{key:'arrowright'}]}}");
  });
});
