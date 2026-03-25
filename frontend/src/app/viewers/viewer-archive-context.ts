export interface ViewerArchiveContext {
  /** Outermost archive on disk, e.g. "drive://c/foo/archive.zip" */
  fsPath: string;
  /** Nested archive chain, e.g. ["inner.tar.gz"] — empty for flat archives. */
  nestedChain: string[];
  /** Entry path inside the innermost archive, e.g. "src/main.go" */
  entryPath: string;
}
