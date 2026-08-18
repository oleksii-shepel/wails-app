import { readFileSync } from 'node:fs';

describe('Reassembly pane intelligence UX', () => {
  const manager = readFileSync(
    new URL('./dual-pane.component.ts', import.meta.url),
    'utf8',
  );

  it('keeps only live modes directly visible', () => {
    expect(manager).toContain('class="live-modes"');
    expect(manager).toContain('>Link</button>');
    expect(manager).toContain('>Mirror</button>');
    expect(manager).toContain('>Scroll</button>');
    expect(manager).toContain('>Follow</button>');
  });

  it('moves secondary actions into the overflow menu', () => {
    expect(manager).toContain('class="pane-intelligence-menu"');
    expect(manager).toContain('Match other pane');
    expect(manager).toContain('Copy to other pane');
    expect(manager).toContain('Previous difference');
    expect(manager).toContain('Save current pair…');
  });

  it('keeps filter mirroring available but not permanently visible', () => {
    expect(manager).toContain("runPaneIntelligenceAction('filter')");
    expect(manager).toContain('Mirror filter');
  });

  it('dismisses the overflow menu when focus context changes', () => {
    expect(manager).toContain('closePaneIntelligenceMenu()');
    expect(manager).toContain("document:mousedown");
  });
});
