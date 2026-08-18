import { paneRelationshipActions } from './pane-relationship-policy';

describe('Reassembly pane relationship actions', () => {
  const base = {
    activeDirectory: true,
    peerDirectory: true,
    activePath: 'C:/src',
    hasCursor: true,
    hasPeer: true,
    linkedRoots: true,
    sourceReadable: true,
    sourceRemovable: true,
    peerWritable: true,
  };

  it('enables the full action set for writable peer panes', () => {
    expect(paneRelationshipActions(base)).toEqual({
      matchOther: true,
      swapRoots: true,
      openPeer: true,
      copyToPeer: true,
      moveToPeer: true,
    });
  });

  it('keeps copy but disables move for a non-removable source', () => {
    const state = paneRelationshipActions({
      ...base,
      sourceRemovable: false,
    });
    expect(state.copyToPeer).toBeTrue();
    expect(state.moveToPeer).toBeFalse();
  });

  it('disables transfers to a read-only peer without disabling Open Peer', () => {
    const state = paneRelationshipActions({
      ...base,
      peerWritable: false,
    });
    expect(state.copyToPeer).toBeFalse();
    expect(state.moveToPeer).toBeFalse();
    expect(state.openPeer).toBeTrue();
  });
});
