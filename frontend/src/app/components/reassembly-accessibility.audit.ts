export interface ReassemblyAccessibilityTarget {
  readonly name: string;
  readonly modal: boolean;
  readonly hasAccessibleName: boolean;
  readonly hasListbox: boolean;
  readonly optionsHaveSelectedState: boolean;
  readonly optionsHavePosition: boolean;
}
export interface ReassemblyAccessibilityAuditResult {
  readonly unnamedModals: readonly string[];
  readonly incompleteListboxes: readonly string[];
}
export function auditReassemblyAccessibility(
  targets: readonly ReassemblyAccessibilityTarget[],
): ReassemblyAccessibilityAuditResult {
  return {
    unnamedModals: targets.filter(target => target.modal && !target.hasAccessibleName).map(target => target.name),
    incompleteListboxes: targets
      .filter(target => target.hasListbox && (!target.optionsHaveSelectedState || !target.optionsHavePosition))
      .map(target => target.name),
  };
}
