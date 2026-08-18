export interface PaneRelationshipActionState {
  readonly matchOther: boolean;
  readonly swapRoots: boolean;
  readonly openPeer: boolean;
  readonly copyToPeer: boolean;
  readonly moveToPeer: boolean;
}

export function paneRelationshipActions(input: {
  activeDirectory: boolean;
  peerDirectory: boolean;
  activePath: string;
  hasCursor: boolean;
  hasPeer: boolean;
  linkedRoots: boolean;
  sourceReadable: boolean;
  sourceRemovable: boolean;
  peerWritable: boolean;
}): PaneRelationshipActionState {
  const usablePanes =
    input.activeDirectory &&
    input.peerDirectory &&
    !!input.activePath;

  return {
    matchOther: usablePanes,
    swapRoots: input.linkedRoots,
    openPeer: usablePanes && input.hasPeer,
    copyToPeer:
      usablePanes &&
      input.hasCursor &&
      input.sourceReadable &&
      input.peerWritable,
    moveToPeer:
      usablePanes &&
      input.hasCursor &&
      input.sourceReadable &&
      input.sourceRemovable &&
      input.peerWritable,
  };
}
