import '@angular/compiler';
import { describe, expect, it, vi } from 'vitest';

import { FileViewContainerComponent } from './file-view-container.component';

function createComponent(): FileViewContainerComponent {
  return new FileViewContainerComponent(
    {} as any,
    {} as any,
    { markForCheck: vi.fn() } as any,
  );
}

describe('FileViewContainerComponent', () => {
  it('prefers the live current path over a stale browser path', () => {
    const component = createComponent();

    component.browserPath = 'https://old.example.com';
    component.currentPath = 'filesystem://c';

    expect(component.getBrowserPath()).toBe('filesystem://c');
    expect(component.showBrowserPane()).toBe(false);
  });

  it('updates the browser pane when a new web address is entered', () => {
    const component = createComponent();

    component.browserPath = 'https://old.example.com';
    component.currentPath = 'https://new.example.com/docs';

    expect(component.getBrowserPath()).toBe('https://new.example.com/docs');
    expect(component.showBrowserPane()).toBe(true);
  });
});