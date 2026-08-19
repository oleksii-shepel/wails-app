import type { FileInfo } from '../shared/protocol';
import type { FilePaneStateController, FilePaneStateHost } from './file-pane-state.controller';

export interface FilePaneStateRuntimeAdapter {
  getPreviewTargetPath(): string;
  emitPreviewTargetFromState(): void;
  getSelectableEntriesPaths(): string[];
  scrollToCursorPath(path: string): void;
  getEffectiveSelection(): Set<string>;
}

export interface FilePaneStateRuntimeSource {
  stateController: FilePaneStateController;
  stateHost: FilePaneStateHost;
  getSelectableEntries(): FileInfo[];
  scrollFileViewToPath(path: string): void;
}

export class FilePaneStateRuntimeFactory {
  create(source: FilePaneStateRuntimeSource): FilePaneStateRuntimeAdapter {
    return {
      getPreviewTargetPath: () => source.stateController.getPreviewTargetPath(source.stateHost),
      emitPreviewTargetFromState: () => source.stateController.emitPreviewTargetFromState(source.stateHost),
      getSelectableEntriesPaths: () => source.getSelectableEntries().map((entry) => entry.path),
      scrollToCursorPath: (path: string) => {
        if (!path) return;
        source.scrollFileViewToPath(path);
      },
      getEffectiveSelection: () => source.stateController.getEffectiveSelection(source.stateHost),
    };
  }
}