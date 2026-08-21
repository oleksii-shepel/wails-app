import { readFileSync } from 'node:fs';

const component = readFileSync(
  new URL('./application-menu-bar.component.ts', import.meta.url),
  'utf8',
);
const app = readFileSync(new URL('../app.component.ts', import.meta.url), 'utf8');

describe('application menu bar architecture', () => {
  it('is a Wails draggable title/menu bar with interactive zones opted out', () => {
    expect(component).toContain('--wails-draggable: drag');
    expect(component).toContain('--wails-draggable: no-drag');
  });

  it('uses the canonical input engine for command state and execution', () => {
    expect(component).toContain('InputEngineService');
    expect(component).toContain('getCommandState(commandId)');
    expect(component).toContain('dispatchCommandById(commandId');
    expect(component).toContain('getPreferredKeyboardBinding(commandId)');
  });

  it('offers the top-level desktop menu groups', () => {
    for (const label of ['File', 'Edit', 'Selection', 'View', 'Go', 'Tools', 'Help']) {
      expect(component).toContain(`label: '${label}'`);
    }
  });

  it('mounts above the dual-pane workspace without stealing its remaining height', () => {
    expect(app).toContain('<app-application-menu-bar></app-application-menu-bar>');
    expect(app).toMatch(/\.app-shell\s*\{[^}]*display:\s*flex;[^}]*flex-direction:\s*column;/s);
    expect(app).toMatch(/\.app-shell\s*>\s*app-dual-pane\s*\{[^}]*flex:\s*1 1 0%;/s);
  });
});