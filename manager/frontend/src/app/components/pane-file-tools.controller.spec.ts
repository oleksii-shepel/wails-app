import { PaneFileToolsController } from './pane-file-tools.controller';

describe('PaneFileToolsController', () => {
  it('keeps metadata and utility behavior outside pane components', () => {
    const names = Object.getOwnPropertyNames(PaneFileToolsController.prototype);
    for (const name of ['revealSelectedInFileManager','touchSelectedModifiedTime','editSelectedAttributes','showSelectedProperties','showSelectedChecksum','calculateSelectedSize','createZipFromSelection']) {
      expect(names).toContain(name);
    }
  });
});
