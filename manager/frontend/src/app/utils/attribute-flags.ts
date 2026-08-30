/** Native attribute flag presentation, mirroring the Permissions column style.
 *
 * The filesystem provider reports Windows attribute flags as concatenated
 * characters in fixed order R, H, S, A (only set flags present — see
 * platform.FormatFileAttributes). This helper renders them positionally with
 * '-' placeholders so the column reads like a permission mask: 'RS' -> 'R-S-'.
 * Unix providers report no attribute characters; empty input stays empty.
 */
const ATTRIBUTE_FLAG_ORDER = ['R', 'H', 'S', 'A'] as const;

export function formatAttributeFlags(value: unknown): string {
  const raw = typeof value === 'string' ? value.trim().toUpperCase() : '';
  if (!raw) return '';
  const order: readonly string[] = ATTRIBUTE_FLAG_ORDER;
  const set = new Set(raw.split('').filter(ch => order.includes(ch)));
  if (set.size === 0) return '';
  return ATTRIBUTE_FLAG_ORDER.map(flag => (set.has(flag) ? flag : '-')).join('');
}
