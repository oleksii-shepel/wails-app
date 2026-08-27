import { readFileSync } from 'node:fs';

describe('pane view VS Code-ish native scrollbars', () => {
  const styles = readFileSync(new URL('../../../styles.scss', import.meta.url), 'utf8');
  const theme = readFileSync(new URL('../../../vscode-theme.scss', import.meta.url), 'utf8');

  it('targets the native scroll owner of every canonical pane view', () => {
    for (const selector of [
      'app-pane-list .pane-content',
      'app-pane-grid .file-grid',
      'app-pane-tree .file-tree-container',
      'app-pane-details .details-body',
    ]) {
      expect(styles).withContext(selector).toContain(selector);
    }
  });

  it('uses a transparent track and rectangular non-inset thumb', () => {
    expect(styles).toContain('background: var(--vsc-scrollbar-track);');
    expect(styles).toContain('border: 0;');
    expect(styles).toContain('border-radius: 0;');
    expect(styles).toContain('background-clip: border-box;');
  });

  it('uses the requested 12px translucent thumb states in both themes', () => {
    expect(theme.match(/--vsc-scrollbar-size: 12px;/g)?.length).toBe(2);
    expect(theme.match(/--vsc-scrollbar-track: transparent;/g)?.length).toBe(2);
    expect(theme.match(/--vsc-scrollbar-thumb: rgba\(121, 121, 121, 0\.42\);/g)?.length).toBe(2);
    expect(theme.match(/--vsc-scrollbar-thumb-hover: rgba\(121, 121, 121, 0\.58\);/g)?.length).toBe(2);
    expect(theme.match(/--vsc-scrollbar-thumb-active: rgba\(121, 121, 121, 0\.72\);/g)?.length).toBe(2);
  });

  it('does not add scrollbar buttons to pane views', () => {
    expect(styles).toContain('::-webkit-scrollbar-button');
    expect(styles).toContain('display: none;');
  });
});
