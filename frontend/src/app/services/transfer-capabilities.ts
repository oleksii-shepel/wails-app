import type { SourceCapabilities } from './source-capabilities';

export type TransferOperation = 'copy' | 'move';

export interface TransferCapabilityDecision {
  readonly allowed: boolean;
  readonly reason?: 'source-unreadable' | 'source-not-removable' | 'destination-read-only';
}

export function canTransferBetween(operation: TransferOperation, source: SourceCapabilities, destination: SourceCapabilities): TransferCapabilityDecision {
  if (!source.read) return { allowed: false, reason: 'source-unreadable' };
  if (!destination.create) return { allowed: false, reason: 'destination-read-only' };
  if (operation === 'move' && !source.delete) return { allowed: false, reason: 'source-not-removable' };
  return { allowed: true };
}

export function transferBlockedMessage(operation: TransferOperation, decision: TransferCapabilityDecision): string {
  const action = operation === 'copy' ? 'Copy' : 'Move';
  switch (decision.reason) {
    case 'source-unreadable': return `${action} Not Available\nThe source cannot be read.`;
    case 'source-not-removable': return `${action} Not Available\nThe source is read-only and cannot be moved. You can copy it instead.`;
    case 'destination-read-only': return `${action} Not Available\nThe destination is read-only.`;
    default: return `${action} Not Available`;
  }
}
