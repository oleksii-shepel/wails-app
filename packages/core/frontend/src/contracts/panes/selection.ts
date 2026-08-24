import type { FileInfo } from '../files/index.js';

export interface PaneSelection {
  readonly cursorEntry: FileInfo | null;
  readonly cursorPath: string | null;
  readonly selectedPaths: ReadonlySet<string>;
  readonly selectedEntries: FileInfo[];
  moveCursor(delta: number, extend?: boolean): void;
  moveCursorHome(extend?: boolean): void;
  moveCursorEnd(extend?: boolean): void;
  moveCursorPage(direction: -1 | 1, extend?: boolean): void;
  selectAll(): void;
  clearSelection(): void;
  toggleCurrentSelection(): void;
  extendSelection(delta: number): void;
  invertSelection(): void;
  selectSameExtension(): void;
  selectSameType(): void;
  selectFiles(): void;
  selectFolders(): void;
  unselectFiles(): void;
  unselectFolders(): void;
  selectByMaskPrompt(): Promise<void>;
  unselectByMaskPrompt(): Promise<void>;
  selectPaths(paths: readonly string[]): void;
  selectComparisonItem(path: string): void;
  reconcileRemovedPaths?(paths: readonly string[]): void;
  recoverVisibleCursor(): void;
  calculateSelectedSize(): Promise<void>;
  selectPeerByName(name: string): boolean;
}