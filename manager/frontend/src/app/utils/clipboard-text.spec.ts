import { restoreGlobals, stubGlobal } from '../testing/jasmine-test-helpers';
import { writeClipboardText } from './clipboard-text';

describe('writeClipboardText', () => {
  afterEach(restoreGlobals);

  it('writes through the browser clipboard when available', async () => {
    const writeText = jasmine.createSpy('writeText').and.resolveTo(undefined);
    const ClipboardSetText = jasmine.createSpy('ClipboardSetText').and.resolveTo(true);
    stubGlobal('navigator', { clipboard: { writeText } });
    stubGlobal('runtime', { ClipboardSetText });

    await writeClipboardText('  exact text\n');

    expect(writeText).toHaveBeenCalledOnceWith('  exact text\n');
    expect(ClipboardSetText).not.toHaveBeenCalled();
  });

  it('uses the native clipboard after the browser denies access', async () => {
    const writeText = jasmine.createSpy('writeText').and.rejectWith(new Error('Permission denied'));
    const ClipboardSetText = jasmine.createSpy('ClipboardSetText').and.resolveTo(true);
    stubGlobal('navigator', { clipboard: { writeText } });
    stubGlobal('runtime', { ClipboardSetText });

    await writeClipboardText('native text');

    expect(ClipboardSetText).toHaveBeenCalledOnceWith('native text');
  });

  it('reports clipboard failure when the native writer returns false', async () => {
    stubGlobal('navigator', {});
    stubGlobal('runtime', { ClipboardSetText: jasmine.createSpy().and.resolveTo(false) });

    await expectAsync(writeClipboardText('unwritten')).toBeRejectedWithError('Clipboard unavailable.');
  });
});
