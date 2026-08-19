import { readFileSync } from 'node:fs';
import { auditDialogFreeze } from './dialog-freeze.audit';

describe('dialog freeze audit', () => {
  const files = [
    '../components/batch-rename-dialog.component.ts',
    '../components/dropdown-container.component.ts',
    '../components/operation-center.component.ts',
    '../components/path-history-viewer.component.ts',
    '../components/remote-connect-dialog.component.ts',
    '../components/selection-basket.component.ts',
    '../components/settings-dialog.component.ts',
    './modal-surface.directive.ts',
  ];

  it('does not let dialog UI acquire NativeOverlayFreezeService', () => {
    const result = auditDialogFreeze(
      files.map(path => ({
        path,
        source: readFileSync(new URL(path, import.meta.url), 'utf8'),
      })),
    );

    expect(result.freezingDialogs).toEqual([]);
  });

  it('keeps the shared modal surface free of native-overlay freezing', () => {
    const source = readFileSync(
      new URL('./modal-surface.directive.ts', import.meta.url),
      'utf8',
    );
    expect(source).not.toContain('NativeOverlayFreezeService');
    expect(source).not.toContain('.acquire(');
    expect(source).not.toContain('.release(');
  });
});
