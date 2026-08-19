import { readFileSync } from 'node:fs';

describe('removed focus-trap references', () => {
  const files = [
    './batch-rename-dialog.component.ts',
    './remote-connect-dialog.component.ts',
    './settings-dialog.component.ts',
  ];

  it('does not reference removed trap directives or utilities', () => {
    for (const file of files) {
      const source = readFileSync(new URL(file, import.meta.url), 'utf8');
      expect(source).not.toContain('DialogFocusTrapDirective');
      expect(source).not.toContain('dialog-focus-trap.directive');
      expect(source).not.toContain('../directives/focus-trap.utils');
      expect(source).not.toContain('appTrapFocus');
    }
  });

  it('uses the neutral focus navigation helper where focus movement is needed', () => {
    const remote = readFileSync(
      new URL('./remote-connect-dialog.component.ts', import.meta.url),
      'utf8',
    );
    const settings = readFileSync(
      new URL('./settings-dialog.component.ts', import.meta.url),
      'utf8',
    );
    expect(remote).toContain('../utils/focus-navigation.utils');
    expect(settings).toContain('../utils/focus-navigation.utils');
  });
});
