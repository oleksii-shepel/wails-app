import { describe, expect, it } from 'vitest';
import { readFileSync } from 'node:fs';

const shared = readFileSync(new URL('./builtins/builtin-viewer.shared.ts', import.meta.url), 'utf8');
const host = readFileSync(new URL('./viewer-host-surface.component.ts', import.meta.url), 'utf8');

describe('viewer production chrome', () => {
  it('defines a final shared chrome derivation', () => {
    expect(shared).toContain('VIEWER_PRODUCTION_CHROME_STYLES');
    expect(shared).toContain(':host-context(app-viewer-host-surface) .viewer-header');
    expect(shared).toContain('height: 28px;');
  });

  it('does not expose implementation viewer keys in host chrome', () => {
    expect(host).not.toContain('<span class="viewer-kind">{{ displayKind }}</span>');
    expect(host).toContain('class="viewer-mode"');
    expect(host).toContain('AUTO');
  });
});
