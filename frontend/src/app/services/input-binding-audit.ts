import type {
  InputBindingDefinition,
  InputContextDefinition,
  KeyStroke,
} from './input-system.types';

export interface InputBindingAuditIssue {
  readonly kind:
    | 'same-context-collision'
    | 'text-key-without-typing-policy';
  readonly bindingIds: readonly string[];
  readonly contextId: string;
  readonly stroke: string;
}

export function auditInputBindings(
  bindings: readonly InputBindingDefinition[],
  contexts: readonly InputContextDefinition[],
): InputBindingAuditIssue[] {
  const issues: InputBindingAuditIssue[] = [];
  const byContextStroke = new Map<string, InputBindingDefinition[]>();

  for (const binding of bindings) {
    if (binding.trigger.device !== 'keyboard') continue;

    const stroke = binding.trigger.chord.steps[0];
    if (!stroke) continue;

    const key = `${binding.contextId}|${formatStroke(stroke)}`;
    const group = byContextStroke.get(key) ?? [];
    group.push(binding);
    byContextStroke.set(key, group);

    if (
      isTextKey(stroke) &&
      !binding.when?.includes('typing') &&
      contexts.some(context =>
        context.id === binding.contextId &&
        (context.kind === 'overlay' || context.kind === 'dialog'),
      )
    ) {
      issues.push({
        kind: 'text-key-without-typing-policy',
        bindingIds: [binding.id],
        contextId: binding.contextId,
        stroke: formatStroke(stroke),
      });
    }
  }

  for (const [key, group] of byContextStroke) {
    if (group.length < 2) continue;

    const [contextId, stroke] = key.split('|', 2);
    issues.push({
      kind: 'same-context-collision',
      bindingIds: group.map(binding => binding.id),
      contextId,
      stroke,
    });
  }

  return issues;
}

function isTextKey(stroke: KeyStroke): boolean {
  return (stroke.modifiers?.length ?? 0) === 0 &&
    /^[a-z0-9]$/i.test(stroke.key);
}

function formatStroke(stroke: KeyStroke): string {
  return [
    ...[...(stroke.modifiers ?? [])].sort(),
    stroke.key.toLocaleLowerCase(),
  ].join('+');
}
