import { readFileSync } from 'node:fs';

describe('parent pseudo-entry cursor', () => {
  const fileEntry = readFileSync(new URL('./file-entry.component.ts', import.meta.url), 'utf8');
  const list = readFileSync(new URL('./pane-list.component.ts', import.meta.url), 'utf8');

  it('passes the program cursor to the virtualized List parent row', () => {
    expect(list).toContain('[cursor]="paneActive && parentEntry.path === cursorPath"');
  });

  it('does not mute away the cursor indicator on the parent pseudo-entry', () => {
    expect(fileEntry).toContain('&.parent-dir');
    expect(fileEntry).toContain('&.cursor {');
    expect(fileEntry).toContain('border-left-color: var(--vsc-accent-blue) !important');
  });
});
