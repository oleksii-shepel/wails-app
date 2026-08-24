import {
  formatLocalDate,
  formatLocalDateTime,
  formatLocalDateTimeInput,
  formatLocalTime,
} from './local-date-time';

describe('local date/time presentation', () => {
  it('formats timestamps using the machine local timezone', () => {
    const date = new Date(2026, 7, 24, 6, 7, 8);
    expect(formatLocalDateTime(date)).toBe(date.toLocaleString([], {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
    }));
    expect(formatLocalDate(date)).toBe(date.toLocaleDateString());
    expect(formatLocalTime(date)).toBe(date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }));
  });

  it('accepts both Unix seconds and JavaScript milliseconds', () => {
    const millis = new Date(2026, 7, 24, 6, 7, 8).getTime();
    expect(formatLocalDateTime(Math.floor(millis / 1000))).toBe(formatLocalDateTime(Math.floor(millis / 1000) * 1000));
  });

  it('creates local editable timestamps without converting through UTC', () => {
    const date = new Date(2026, 7, 24, 6, 7, 8);
    expect(formatLocalDateTimeInput(date)).toBe('2026-08-24T06:07:08');
  });
});
