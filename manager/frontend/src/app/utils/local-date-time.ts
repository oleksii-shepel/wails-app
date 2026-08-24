/** GUI-only local date/time presentation helpers.
 *
 * Internal timestamps stay canonical. These helpers are the single boundary
 * where timestamps become user-visible local date/time strings.
 */
export function localDate(value: unknown): Date | null {
  if (value instanceof Date) {
    return Number.isNaN(value.getTime()) ? null : value;
  }

  if (typeof value === 'number') {
    if (!Number.isFinite(value) || value <= 0) return null;
    const millis = Math.abs(value) < 10_000_000_000 ? value * 1000 : value;
    const date = new Date(millis);
    return Number.isNaN(date.getTime()) ? null : date;
  }

  if (typeof value === 'string') {
    const trimmed = value.trim();
    if (!trimmed) return null;
    const numeric = Number(trimmed);
    if (Number.isFinite(numeric) && /^[-+]?\d+(?:\.\d+)?$/.test(trimmed)) {
      return localDate(numeric);
    }
    const date = new Date(trimmed);
    return Number.isNaN(date.getTime()) ? null : date;
  }

  return null;
}

export function formatLocalDateTime(value: unknown, fallback = ''): string {
  const date = localDate(value);
  if (!date) return fallback;
  return date.toLocaleString([], {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  });
}

export function formatLocalDate(value: unknown, fallback = ''): string {
  const date = localDate(value);
  return date ? date.toLocaleDateString() : fallback;
}

export function formatLocalTime(value: unknown, fallback = ''): string {
  const date = localDate(value);
  return date
    ? date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
    : fallback;
}

/** YYYY-MM-DDTHH:mm:ss in the machine's local timezone, for native/local inputs. */
export function formatLocalDateTimeInput(value: unknown = Date.now()): string {
  const date = localDate(value) ?? new Date();
  const pad = (part: number) => String(part).padStart(2, '0');
  return [
    date.getFullYear(),
    '-', pad(date.getMonth() + 1),
    '-', pad(date.getDate()),
    'T', pad(date.getHours()),
    ':', pad(date.getMinutes()),
    ':', pad(date.getSeconds()),
  ].join('');
}
