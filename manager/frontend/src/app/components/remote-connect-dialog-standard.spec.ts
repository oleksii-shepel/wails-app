import { readFileSync } from 'node:fs';

const component = readFileSync(
  new URL('./remote-connect-dialog.component.ts', import.meta.url),
  'utf8',
);
const componentStyles = readFileSync(
  new URL('./remote-connect-dialog.component.scss', import.meta.url),
  'utf8',
);
const toolsTheme = readFileSync(
  new URL('./tools-dialog-remote-theme.scss', import.meta.url),
  'utf8',
);
const commandPalette = readFileSync(
  new URL('./command-palette.component.ts', import.meta.url),
  'utf8',
);

describe('Connect Remote standard dialog contract', () => {
  it('uses the shared dialog animation classes and completes the queued close on animation end', () => {
    expect(component).toContain('class=\"remote-connect-overlay palette-dialog-backdrop dialog-group\"');
    expect(component).toContain('[class.dialog-group--closing]=\"isClosing\"');
    expect(component).toContain('class=\"remote-connect-panel palette-dialog-surface dialog-surface\"');
    expect(component).toContain('this.remoteConnectDialog.finishClose();');
  });

  it('does not let the legacy Tools animation family override Connect Remote', () => {
    expect(toolsTheme).not.toContain('.remote-connect-overlay');
    expect(toolsTheme).not.toContain('.remote-connect-panel');
    expect(toolsTheme).not.toContain('.remote-connect-header');
  });

  it('uses the same close-button class and local spacing as Command Palette', () => {
    expect(component).toContain('class=\"dialog-close-btn\"');
    expect(component).toContain('title=\"Close (Esc)\"');
    expect(commandPalette).toContain('class=\"dialog-close-btn\"');
    expect(commandPalette).toContain('.dialog-close-btn{margin-left:8px}');
    expect(componentStyles).toMatch(/\.remote-connect-panel \.dialog-close-btn \{\s*margin-left: 8px;\s*\}/);
    expect(componentStyles).not.toMatch(/\.dialog-close-btn[^}]*padding:/s);
    expect(componentStyles).not.toMatch(/\.dialog-close-btn[^}]*min-height:/s);
  });
});
