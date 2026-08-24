import { readFileSync } from 'node:fs';

const read = (name: string) =>
  readFileSync(new URL(`./${name}`, import.meta.url), 'utf8');

describe('pane source observation wiring', () => {
  it('activates the Bin observer and closes it with the pane', () => {
    const source = read('bin-pane.component.ts');
    expect(source).toContain('this.sourceChanges.observe(');
    expect(source).toContain(
      'this.sourceObservation.setLocation(BIN_ROOT_PATH);',
    );
    expect(source).toContain('this.sourceObservation?.close();');
  });

  it('updates filesystem observation after each successful Directory load', () => {
    const source = read('directory-pane.component.ts');
    expect(source).toContain('this.sourceChanges.observe(');
    expect(source).toContain(
      'this.sourceObservation?.setLocation(nextPath);',
    );
    expect(source).toContain('this.sourceObservation?.close();');
  });
});
