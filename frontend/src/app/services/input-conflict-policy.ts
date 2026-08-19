import type { InputBindingDefinition } from './input-system.types';

/**
 * Returns true when two simple conjunctive `when` clauses may both evaluate to
 * true for the same input event. Unknown expressions are treated conservatively
 * as overlapping so the conflict inspector never hides a real collision.
 */
export function whenClausesCanOverlap(left?: string, right?: string): boolean {
  const leftTerms = parseConjunction(left);
  const rightTerms = parseConjunction(right);

  if (!leftTerms || !rightTerms) return true;

  if (leftTerms.has('__never__') || rightTerms.has('__never__')) return false;

  for (const [token, polarity] of leftTerms) {
    const peer = rightTerms.get(token);
    if (peer !== undefined && peer !== polarity) return false;
  }

  return true;
}

/**
 * Stable final tie-breaker for semantically equal bindings. Registration order
 * is intentionally excluded: plugin/component load timing must never change
 * which command owns a shortcut.
 */
export function compareStableBindingIdentity(
  left: Pick<InputBindingDefinition, 'id' | 'commandId'>,
  right: Pick<InputBindingDefinition, 'id' | 'commandId'>,
): number {
  if (left.id !== right.id) return left.id < right.id ? -1 : 1;
  if (left.commandId === right.commandId) return 0;
  return left.commandId < right.commandId ? -1 : 1;
}

function parseConjunction(expression?: string): Map<string, boolean> | null {
  const normalized = expression?.trim();
  if (!normalized || normalized === 'true') return new Map();

  const result = new Map<string, boolean>();
  const terms = normalized.split('&&').map(term => term.trim()).filter(Boolean);
  if (terms.length === 0) return new Map();

  for (const term of terms) {
    // The live input language is intentionally a conjunction of boolean tokens.
    // If richer syntax appears later, report overlap rather than guessing.
    if (term.includes('||') || term.includes('(') || term.includes(')')) return null;

    const negated = term.startsWith('!');
    const token = (negated ? term.slice(1) : term).trim();
    if (!/^[A-Za-z][A-Za-z0-9]*$/.test(token)) return null;

    const polarity = !negated;
    const existing = result.get(token);
    if (existing !== undefined && existing !== polarity) {
      // An internally contradictory clause can never be active.
      return new Map([['__never__', true]]);
    }
    result.set(token, polarity);
  }

  return result;
}
