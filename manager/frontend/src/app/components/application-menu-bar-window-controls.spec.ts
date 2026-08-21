import { readFileSync } from 'node:fs';

const source = readFileSync(
  new URL('./application-menu-bar.component.ts', import.meta.url),
  'utf8',
);

describe('ApplicationMenuBar window controls', () => {
  it('keeps the title bar draggable while controls remain interactive', () => {
    expect(source).toContain('--wails-draggable: drag');
    expect(source).toContain('.window-controls *');
    expect(source).toContain('--wails-draggable: no-drag');
  });

  it('provides minimize, maximize/restore and close controls', () => {
    expect(source).toContain('WindowMinimise()');
    expect(source).toContain('WindowToggleMaximise()');
    expect(source).toContain('WindowIsMaximised()');
    expect(source).toContain('Quit()');
  });

  it('persists window geometry before close', () => {
    expect(source).toContain('this.windowState.captureNow()');
  });

  it('renders the restore glyph while maximized', () => {
    expect(source).toContain("*ngIf="maximized"");
    expect(source).toContain("maximized ? 'Restore' : 'Maximize'");
  });
});
