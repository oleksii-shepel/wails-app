import { readFileSync } from 'node:fs';

const details = readFileSync(new URL('./pane-details.component.ts', import.meta.url), 'utf8');
const protocol = readFileSync(new URL('../shared/protocol.ts', import.meta.url), 'utf8');

describe('Details security metadata', () => {
  it('shows owner and the complete permission field in Details', () => {
    expect(details).toContain("field: 'owner', header: 'Owner'");
    expect(details).toContain("field: 'permissions', header: 'Permissions'");
    expect(details).toContain("field: 'attributes', header: 'Attributes'");
    expect(details).toContain('formatter: (value) => formatAttributeFlags(value)');
    expect(protocol).toContain('owner?: string;');
    expect(protocol).toContain('attributes?: string;');
  });

  it('also exposes the remaining core timestamps and file flags', () => {
    for (const field of ['created', 'accessed', 'isHidden', 'isSymlink']) {
      expect(details).withContext(field).toContain(`field: '${field}'`);
    }
  });
});
