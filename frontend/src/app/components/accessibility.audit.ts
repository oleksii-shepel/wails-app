export interface AccessibilityTarget {
  readonly name: string;
  readonly modal: boolean;
  readonly hasAccessibleName: boolean;
  readonly hasListbox: boolean;
  readonly optionsHaveSelectedState: boolean;
  readonly optionsHavePosition: boolean;
}
export interface AccessibilityAuditResult {
  readonly unnamedModals: readonly string[];
  readonly incompleteListboxes: readonly string[];
}
export function auditReassemblyAccessibility(
  targets: readonly AccessibilityTarget[],
): AccessibilityAuditResult {
  return {
    unnamedModals: targets.filter(target => target.modal && !target.hasAccessibleName).map(target => target.name),
    incompleteListboxes: targets
      .filter(target => target.hasListbox && (!target.optionsHaveSelectedState || !target.optionsHavePosition))
      .map(target => target.name),
  };
}
