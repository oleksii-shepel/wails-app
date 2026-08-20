import { PaneFileCommandsController } from './pane-file-commands.controller';

describe('PaneFileCommandsController', () => {
  it('keeps creation/manipulation behavior outside pane components', () => {
    expect(PaneFileCommandsController).toBeDefined();
    const names = Object.getOwnPropertyNames(PaneFileCommandsController.prototype);
    expect(names).toContain('newFile');
    expect(names).toContain('newDocumentTemplate');
    expect(names).toContain('duplicateSelected');
    expect(names).toContain('moveSelectionToNewFolder');
    expect(names).toContain('createLinkForSelected');
    expect(names).toContain('createShortcutForSelected');
  });
});
