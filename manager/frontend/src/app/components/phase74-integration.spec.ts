import { readFileSync } from 'node:fs';

describe('Phase 74 input integration', () => {
  const manager = readFileSync(
    new URL('./dual-pane.component.ts', import.meta.url),
    'utf8',
  );
  const editor = readFileSync(
    new URL('./input-center.component.ts', import.meta.url),
    'utf8',
  );
  const engine = readFileSync(
    new URL('../services/input-engine.service.ts', import.meta.url),
    'utf8',
  );
  const loader = readFileSync(
    new URL('../services/directory-plugin-package-loader.service.ts', import.meta.url),
    'utf8',
  );

  it('mounts the live keyboard/mouse editor', () => {
    expect(manager).toContain('<app-input-center');
    expect(editor).toContain('Input & Shortcuts');
    expect(editor).toContain('conflictsOnly');
    expect(editor).toContain('beginCapture(binding)');
  });

  it('adds a true user override layer and conflict inspection', () => {
    expect(engine).toContain('applyUserBindingCustomization(');
    expect(engine).toContain('clearUserBindingCustomization(');
    expect(engine).toContain('inspectBindingConflicts()');
    expect(engine).toContain("source: 'user'");
  });

  it('makes drag gestures reachable from real mouse input', () => {
    expect(engine).toContain("document.addEventListener('mousemove'");
    expect(engine).toContain("type: 'drag'");
    expect(engine).toContain('distance < 8');
  });

  it('suspends normal dispatch while recording a replacement trigger', () => {
    expect(editor).toContain('this.engine.setKeyboardEnabled(false)');
    expect(editor).toContain('this.engine.setKeyboardEnabled(true)');
  });

  it('cleans runtime user overrides when a plugin unloads', () => {
    expect(engine).toContain('this.clearUserBindingCustomization(bindingId)');
  });

  it('lets declarative directory plugins contribute bindings', () => {
    expect(loader).toContain('module.bindings');
    expect(loader).toContain('context.api.registerInput({');
    expect(loader).toContain("source: 'plugin'");
  });

  it('adds initial Reassembly mouse command bindings', () => {
    expect(manager).toContain('reassembly.mouse.file.doubleClick');
    expect(manager).toContain('reassembly.mouse.viewer.ctrlWheelUp');
    expect(manager).toContain('reassembly.mouse.viewer.ctrlWheelDown');
  });

  it('keeps Phase 73 build fixes intact', () => {
    expect(manager).not.toContain("item.status !== 'equal'");
    expect(manager).not.toContain("case 'different':");
    expect(manager).not.toContain('targetPane.load(peer');
  });
});