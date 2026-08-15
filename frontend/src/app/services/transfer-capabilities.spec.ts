import type { SourceCapabilities } from './source-capabilities';
import { canReceiveTransfer, canTransferBetween } from './transfer-capabilities';
const mutable: SourceCapabilities = { read: true, create: true, delete: true, rename: true, trash: false };
const readOnly: SourceCapabilities = { read: true, create: false, delete: false, rename: false, trash: false };
describe('transfer capabilities', () => {
  it('requires create capability on any transfer destination', () => {
    expect(canReceiveTransfer(mutable).allowed).toBe(true);
    expect(canReceiveTransfer(readOnly)).toEqual({ allowed: false, reason: 'destination-read-only' });
  });

  it('allows copying from a read-only source into a writable destination', () => expect(canTransferBetween('copy', readOnly, mutable)).toEqual({ allowed: true }));
  it('blocks copying into a read-only destination', () => expect(canTransferBetween('copy', mutable, readOnly)).toEqual({ allowed: false, reason: 'destination-read-only' }));
  it('blocks moving from a read-only source', () => expect(canTransferBetween('move', readOnly, mutable)).toEqual({ allowed: false, reason: 'source-not-removable' }));
  it('allows moving between mutable sources', () => expect(canTransferBetween('move', mutable, mutable)).toEqual({ allowed: true }));
});

