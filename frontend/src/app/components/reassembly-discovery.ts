export interface DiscoveryField {
  value: string | null | undefined;
  weight?: number;
}

export function discoveryTokens(query: string): string[] {
  return normalize(query).split(/\s+/).filter(Boolean);
}

export function discoveryScore(
  query: string,
  fields: readonly DiscoveryField[],
): number {
  const tokens = discoveryTokens(query);
  if (tokens.length === 0) return 0;

  let total = 0;

  for (const token of tokens) {
    let best = -1;

    for (const field of fields) {
      const value = normalize(field.value);
      if (!value) continue;

      const score = tokenScore(value, token) * (field.weight ?? 1);
      if (score > best) best = score;
    }

    if (best < 0) return -1;
    total += best;
  }

  return total;
}

function tokenScore(value: string, token: string): number {
  if (value === token) return 120;
  if (value.startsWith(token)) return 100 - lengthPenalty(value, token);

  const wordIndex = value.search(
    new RegExp(`(?:^|[\\s/_\\\\.-])${escapeRegExp(token)}`),
  );
  if (wordIndex >= 0) return 85 - Math.min(20, wordIndex);

  const includes = value.indexOf(token);
  if (includes >= 0) return 65 - Math.min(25, includes);

  let cursor = 0;
  let gaps = 0;
  for (const character of token) {
    const next = value.indexOf(character, cursor);
    if (next < 0) return -1;
    gaps += next - cursor;
    cursor = next + 1;
  }

  return Math.max(1, 35 - Math.min(30, gaps));
}

function lengthPenalty(value: string, token: string): number {
  return Math.min(20, Math.max(0, value.length - token.length) / 3);
}

function normalize(value: string | null | undefined): string {
  return String(value ?? '').trim().toLocaleLowerCase();
}

function escapeRegExp(value: string): string {
  return value.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}
