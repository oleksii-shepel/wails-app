import { readFileSync } from 'node:fs';

describe('pane selection persistence on deactivation', () => {
  const paneFiles = [
    'directory-pane.component.ts',
    'bin-pane.component.ts',
    'registry-pane.component.ts',
    'autorun-pane.component.ts',
  ];

  for (const name of paneFiles) {
    it(`${name}: clears only a lone selection when pane becomes inactive`, () => {
      const source = readFileSync(new URL(`./${name}`, import.meta.url), 'utf8');

      expect(source).toContain('if (wasActive && !next)');
      expect(source).toContain('this.onPaneDeactivated();');
      expect(source).toContain('if (this.selectedPaths.size <= 1)');
      expect(source).toContain('this.selectedPaths = new Set<string>();');

      const methodStart = source.indexOf('private onPaneDeactivated(): void');
      const methodEnd = source.indexOf('\n  private pointerActivationPending', methodStart);
      const method = source.slice(methodStart, methodEnd);

      // Cursor survives; only single selection and anchor are cleared.
      expect(method).not.toContain('this.cursorPath = null');
      expect(method).toContain('this.anchorPath = null');
    });
  }
});
