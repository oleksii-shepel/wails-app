import '@angular/compiler';
import { SimpleChange } from '@angular/core';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { FileViewContainerComponent } from './file-view-container.component';

function createComponent(): FileViewContainerComponent {
  return new FileViewContainerComponent(
    {} as any,
    {} as any,
    { markForCheck: vi.fn() } as any,
  );
}

function applyPathChanges(component: FileViewContainerComponent): void {
  component.ngOnChanges({
    browserPath: new SimpleChange('', component.browserPath, true),
    currentPath: new SimpleChange('', component.currentPath, true),
  });
}

function createLocalStorageMock(initial: Record<string, string> = {}) {
  const store = new Map(Object.entries(initial));

  return {
    getItem: vi.fn((key: string) => store.get(key) ?? null),
    setItem: vi.fn((key: string, value: string) => {
      store.set(key, value);
    }),
    removeItem: vi.fn((key: string) => {
      store.delete(key);
    }),
    clear: vi.fn(() => {
      store.clear();
    }),
  };
}

describe('FileViewContainerComponent', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('prefers the live current path over a stale browser path', () => {
    const component = createComponent();

    component.browserPath = 'https://old.example.com';
    component.currentPath = 'filesystem://c';
    applyPathChanges(component);

    expect(component.getBrowserPath()).toBe('filesystem://c');
    expect(component.showBrowserPane()).toBe(false);
  });

  it('updates the browser pane when a new web address is entered', () => {
    const component = createComponent();

    component.browserPath = 'https://old.example.com';
    component.currentPath = 'https://new.example.com/docs';
    applyPathChanges(component);

    expect(component.getBrowserPath()).toBe('https://new.example.com/docs');
    expect(component.showBrowserPane()).toBe(true);
  });

  it('loads and clamps the persisted browser pane split for the current pane', () => {
    vi.stubGlobal('localStorage', createLocalStorageMock({
      'fileManager_browserPaneSplit:right': '90',
    }));

    const component = createComponent();
    component.paneId = 'right';
    component.ngOnChanges({
      paneId: new SimpleChange('left', 'right', true),
    });

    expect(component.browserPaneSplit).toBe(72);
  });
});