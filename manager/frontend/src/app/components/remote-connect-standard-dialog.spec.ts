import { readFileSync } from 'node:fs';

const component = readFileSync(new URL('./remote-connect-dialog.component.ts', import.meta.url), 'utf8');
const styles = readFileSync(new URL('./remote-connect-dialog.component.scss', import.meta.url), 'utf8');
const legacyTheme = readFileSync(new URL('./tools-dialog-remote-theme.scss', import.meta.url), 'utf8');
const globalStyles = readFileSync(new URL('../../../styles.scss', import.meta.url), 'utf8');

describe('Connect Remote standard dialog contract', () => {
  it('uses the standard close button without visible Escape helper text', () => {
    expect(component).toContain('class="dialog-close"');
    expect(component).not.toContain('Esc to close');
    expect(styles).not.toMatch(/\.remote-connect-panel\s+\.dialog-close/);
  });

  it('uses only the shared dialog animation family', () => {
    expect(component).toContain('class="remote-connect-overlay palette-dialog-backdrop dialog-group"');
    expect(component).toContain('[class.dialog-group--closing]="isClosing"');
    expect(legacyTheme).not.toMatch(/remote-connect-overlay[^}]*animation:/s);
    expect(legacyTheme).not.toMatch(/remote-connect-panel[^}]*animation:/s);
  });


  it('centers the dialog and keeps the shared close button neutral while pressed', () => {
    expect(styles).toMatch(/\.remote-connect-overlay\s*\{[^}]*place-items:\s*center;/s);
    expect(globalStyles).toMatch(/\.dialog-close:active,[\s\S]*background:\s*var\(--vsc-list-hover-background/);
    expect(globalStyles).toContain('.dialog-close:focus:not(:focus-visible)');
  });

  it('avoids form-column vertical scrolling at normal desktop sizes', () => {
    expect(styles).toContain('max-height: calc(100vh - 24px)');
    expect(styles).toMatch(/\.remote-connect-form-section,[\s\S]*overflow: visible;/);
  });
});