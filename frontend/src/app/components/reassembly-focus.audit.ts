export interface ReassemblyFocusAuditTarget {
  readonly name: string;
  readonly modal: boolean;
  readonly hasSharedModalSurface: boolean;
  readonly hasManualFocusRestore: boolean;
  readonly hasManualTabTrap: boolean;
}
export interface ReassemblyFocusAuditResult {
  readonly duplicateFocusOwners: readonly string[];
  readonly missingModalPolicy: readonly string[];
}
export function auditReassemblyFocus(targets: readonly ReassemblyFocusAuditTarget[]): ReassemblyFocusAuditResult {
  return {
    duplicateFocusOwners: targets
      .filter(target => target.hasSharedModalSurface && (target.hasManualFocusRestore || target.hasManualTabTrap))
      .map(target => target.name),
    missingModalPolicy: targets
      .filter(target => target.modal && !target.hasSharedModalSurface)
      .map(target => target.name),
  };
}
