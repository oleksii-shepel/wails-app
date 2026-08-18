// Lightweight structural audit for reassembly input wiring.
// This intentionally has no Angular dependency and can be copied into a test
// or build script. It documents invariants the reassembly must preserve.

export interface AuditableBinding {
  id: string;
  commandId: string;
}

export interface InputAuditResult {
  duplicateBindingIds: string[];
  bindingsWithoutCommands: string[];
  commandsWithoutDescriptors: string[];
}

export function auditInputCatalog(
  bindings: readonly AuditableBinding[],
  commandIds: readonly string[],
  descriptorIds: readonly string[],
): InputAuditResult {
  const duplicateBindingIds = duplicates(bindings.map(binding => binding.id));
  const commandSet = new Set(commandIds);
  const descriptorSet = new Set(descriptorIds);

  return {
    duplicateBindingIds,
    bindingsWithoutCommands: unique(
      bindings
        .map(binding => binding.commandId)
        .filter(commandId => !commandSet.has(commandId)),
    ),
    commandsWithoutDescriptors: unique(
      commandIds.filter(commandId => !descriptorSet.has(commandId)),
    ),
  };
}

function duplicates(values: readonly string[]): string[] {
  const counts = new Map<string, number>();
  for (const value of values) {
    counts.set(value, (counts.get(value) ?? 0) + 1);
  }
  return [...counts]
    .filter(([, count]) => count > 1)
    .map(([value]) => value)
    .sort();
}

function unique(values: readonly string[]): string[] {
  return [...new Set(values)].sort();
}
