import { Pipe, PipeTransform } from '@angular/core';
import { formatLocalDate, formatLocalDateTime, formatLocalTime } from './local-date-time';

export type LocalDateTimeDisplay = 'dateTime' | 'date' | 'time';

@Pipe({
  name: 'localDateTime',
  standalone: true,
  pure: true,
})
export class LocalDateTimePipe implements PipeTransform {
  transform(value: unknown, display: LocalDateTimeDisplay = 'dateTime', fallback = ''): string {
    if (display === 'date') return formatLocalDate(value, fallback);
    if (display === 'time') return formatLocalTime(value, fallback);
    return formatLocalDateTime(value, fallback);
  }
}
