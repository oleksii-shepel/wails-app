import { readFileSync } from 'node:fs';

const source = readFileSync(
  new URL('./network-pane.component.ts', import.meta.url),
  'utf8',
);

describe('Network pane status line', () => {
  it('renders errors in the line below the address bar', () => {
    expect(source).toContain('[class.status-error]="!!error"');
    expect(source).toContain('<span class="status-message" [title]="error">{{ error }}</span>');
    expect(source).toContain('class="status-action"');
  });

  it('does not replace the pane body with a full-screen error page', () => {
    expect(source).not.toContain('class="error" *ngIf="error"');
  });

  it('uses the same line for active discovery', () => {
    expect(source).toContain('[class.status-running]="loading"');
    expect(source).toContain('{{ networkLoadingText }}');
    expect(source).toContain('network-discovery-spinner');
  });

  it('keeps retry/rescan inline', () => {
    expect(source).toContain('{{ networkRetryLabel }}');
    expect(source).toContain('(click)="refresh()"');
  });

  it('does not hide the body while error is present', () => {
    expect(source).not.toContain('*ngIf="!error && viewMode');
  });
});
