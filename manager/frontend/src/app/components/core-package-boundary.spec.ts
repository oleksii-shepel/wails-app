import { readFileSync } from 'node:fs';

const paneContract = readFileSync(new URL('./pane-component.contract.ts', import.meta.url), 'utf8');
const sourceContract = readFileSync(new URL('../services/panel-source.ts', import.meta.url), 'utf8');
const renderer = readFileSync(new URL('./pane-renderer-registry.service.ts', import.meta.url), 'utf8');

describe('@reassembly/core boundary', () => {
  it('uses domain-oriented contract entry points', () => {
    expect(paneContract).toContain("from '@reassembly/core/contracts/panes'");
    expect(sourceContract).toContain("from '@reassembly/core/contracts/sources'");
  });

  it('keeps Angular DI outside the framework-neutral pane runtime', () => {
    expect(renderer).toContain("from '@reassembly/core/runtime/panes'");
    expect(renderer).toContain('@Injectable');
  });
});
