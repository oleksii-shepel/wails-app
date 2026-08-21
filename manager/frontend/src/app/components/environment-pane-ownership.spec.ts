import { readFileSync } from 'node:fs';

const directory = readFileSync(
  new URL('./directory-pane.component.ts', import.meta.url),
  'utf8',
);
const environment = readFileSync(
  new URL('./environment-pane.component.ts', import.meta.url),
  'utf8',
);
const host = readFileSync(
  new URL('./pane-host.component.ts', import.meta.url),
  'utf8',
);
const renderers = readFileSync(
  new URL('./pane-renderer-registry.service.ts', import.meta.url),
  'utf8',
);

describe('Manager-owned Environment pane', () => {
  it('is derived from the Directory pane implementation shape', () => {
    expect(environment).toContain("selector: 'app-environment-pane'");
    expect(environment).toContain('export class EnvironmentPaneComponent');
    for (const token of [
      'class="pane"',
      'class="tab-strip"',
      'class="navigation-bar"',
      'class="status-row"',
      'class="action-bar"',
      'class="details-header"',
      'class="list"',
      'class="entry"',
    ]) {
      expect(environment).toContain(token);
      expect(directory).toContain(token);
    }
  });

  it('owns Environment Details columns', () => {
    expect(environment).toContain(
      '<span></span><span>Name</span><span>Scope</span><span>Value</span><span>Type</span>',
    );
    expect(environment).toContain("environmentField(entry, 'scope')");
    expect(environment).toContain("environmentField(entry, 'value')");
    expect(environment).toContain("environmentField(entry, 'valueType')");
  });

  it('starts Details-only and does not expose filesystem mutation actions', () => {
    expect(environment).toContain("viewMode: DirectoryView = 'details';");
    expect(environment).not.toContain('ariaLabel="Directory actions"');
    expect(environment).toContain('[actions]="environmentToolbarActions"');
    expect(environment).not.toContain("id: 'directory.new-folder'");
    expect(environment).not.toContain("id: 'directory.delete'");
  });

  it('has a first-class renderer and standard-pane host path', () => {
    expect(renderers).toContain("| 'environment'");
    expect(renderers).toContain("id: 'builtin.environment-pane'");
    expect(renderers).not.toMatch(
      /SYSTEM_TABLE_SCHEMES[\s\S]*?'environment'[\s\S]*?@Injectable/,
    );
    expect(host).toContain(
      "import { EnvironmentPaneComponent } from './environment-pane.component';",
    );
    expect(host).toContain("*ngSwitchCase=\"'environment'\"");
    expect(host).toContain('<app-environment-pane');
  });
});
