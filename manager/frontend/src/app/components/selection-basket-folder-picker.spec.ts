import { readFileSync } from 'node:fs';

const basket = readFileSync(new URL('./selection-basket.component.ts', import.meta.url), 'utf8');
const dual = readFileSync(new URL('./dual-pane.component.ts', import.meta.url), 'utf8');
const api = readFileSync(new URL('../services/api.service.ts', import.meta.url), 'utf8');
const backend = readFileSync(
  new URL('../../../../internal/app/bindings_filesystem.go', import.meta.url),
  'utf8',
);

describe('Selection Basket folder destination', () => {
  it('offers a native folder picker next to pane presets', () => {
    expect(basket).toContain('(click)="browseDestination()"');
    expect(basket).toContain("{{ pickingDestination ? 'Opening…' : 'Browse…' }}");
    expect(basket).toContain('<option *ngIf="customDestination" value="custom">');
  });

  it('uses the chosen folder even when pane destinations are unavailable', () => {
    expect(basket).toContain("destinationMode === 'custom'");
    expect(basket).toContain('destinationPath: this.destinationMode ===');
    expect(dual).toContain('if (request.destinationPath)');
    expect(dual).toContain('transferBasketToPath(');
  });

  it('routes folder selection through the native Wails directory dialog', () => {
    expect(api).toContain('window.go.app.App.ChooseDirectory(');
    expect(backend).toContain('wailsruntime.OpenDirectoryDialog(');
    expect(backend).toContain('Title:            "Choose destination folder"');
  });

  it('validates that the selected destination is still a directory before transfer', () => {
    expect(dual).toContain('const info = await this.api.getFileInfo(path);');
    expect(dual).toContain('if (info.type !== FileType.DIRECTORY)');
  });
});
