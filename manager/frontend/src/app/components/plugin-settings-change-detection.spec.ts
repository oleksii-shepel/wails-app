import { readFileSync } from 'node:fs';

describe('Plugin settings change detection', () => {
  const source = readFileSync(
    new URL('./plugin-settings-panel.component.ts', import.meta.url),
    'utf8',
  );

  it('renders plugin state immediately after Wails-backed async work settles', () => {
    expect(source).toContain('ChangeDetectorRef');
    expect(source).toContain('this.changeDetector.detectChanges()');
    expect(source).toContain('this.busyIds.delete(id);\n      this.renderNow();');
    expect(source).toContain('this.refreshing = false;\n      this.renderNow();');
  });

  it('does not depend on opening the plugin directory to refresh snapshots', () => {
    const runFor = source.slice(
      source.indexOf('private async runFor('),
      source.indexOf('private renderNow()'),
    );
    expect(runFor).toContain('this.snapshots = await withTimeout(');
    expect(runFor).not.toContain('openPluginDirectory');
  });
});
