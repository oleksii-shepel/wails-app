import { readFileSync } from 'node:fs';
import { TestBed } from '@angular/core/testing';
import { ChangeDetectorRef, ElementRef, NgZone } from '@angular/core';
import { ModalStackService } from '../services/modal-stack.service';
import { CommandRegistryService } from '../services/command-registry.service';
import { InputEngineService } from '../services/input-engine.service';
import { InputCenterComponent } from './input-center.component';
import { WorkspaceSwitcherComponent } from './workspace-switcher.component';
import { ContextMenuComponent } from './context-menu.component';
import { FileContextMenuComponent } from './file-context-menu.component';
import { DropdownDirective } from '../directives/dropdown.directive';
import { DropdownService } from '../services/dropdown.service';
import { InputCustomizationService } from '../services/input-customization.service';
import { ConfirmDialogService } from '../services/confirm-dialog.service';
import { WorkspaceService } from '../services/workspace.service';
import { WorkspaceSurfaceService } from '../services/workspace-surface.service';

const readComponent = (name: string) =>
  readFileSync(new URL(`./${name}`, import.meta.url), 'utf8');

describe('auxiliary window Escape handling', () => {
  afterEach(() => {
    TestBed.resetTestingModule();
  });

  it('registers every dialog component with ModalStackService', () => {
    const dialogComponents = [
      'about-dialog.component.ts',
      'archive-files-dialog.component.ts',
      'batch-rename-dialog.component.ts',
      'closed-tabs.component.ts',
      'command-palette.component.ts',
      'confirm-dialog.component.ts',
      'disk-usage-dialog.component.ts',
      'file-diff-dialog.component.ts',
      'global-search.component.ts',
      'input-center.component.ts',
      'keyboard-help.component.ts',
      'operation-center.component.ts',
      'operation-journal.component.ts',
      'pane-compare-dialog.component.ts',
      'pane-recursive-compare-dialog.component.ts',
      'pane-sync-review.component.ts',
      'path-history.component.ts',
      'path-history-viewer.component.ts',
      'quick-access.component.ts',
      'remote-connect-dialog.component.ts',
      'remote-connections.component.ts',
      'selection-basket.component.ts',
      'settings-dialog.component.ts',
      'tools-center.component.ts',
      'trash-dialog.component.ts',
      'workspace-switcher.component.ts',
    ];

    for (const file of dialogComponents) {
      const source = readComponent(file);
      expect(source).withContext(file).toContain('modal-stack.service');
      expect(source).withContext(file).toContain('register(');
    }
  });

  it('yields InputEngine plain Escape when a different modal layer is topmost', () => {
    const zone = new NgZone({ enableLongStackTrace: false });
    TestBed.configureTestingModule({
      providers: [
        ModalStackService,
        { provide: NgZone, useValue: zone },
        { provide: CommandRegistryService, useFactory: () => new CommandRegistryService(zone) },
        {
          provide: InputEngineService,
          useFactory: () => new InputEngineService(zone, TestBed.inject(CommandRegistryService)),
        },
      ],
    });

    const modalStack = TestBed.inject(ModalStackService);
    const engine = TestBed.inject(InputEngineService);

    let underlayClosed = false;
    let menuClosed = false;

    engine.registerContext({
      id: 'overlay.underlay',
      kind: 'overlay',
      priority: 400,
      exclusive: true,
      parentId: 'global',
    });
    engine.registerCommand('overlay.underlay.close', () => {
      underlayClosed = true;
    }, 'test');
    engine.registerBinding({
      id: 'overlay.underlay.escape',
      commandId: 'overlay.underlay.close',
      contextId: 'overlay.underlay',
      source: 'default',
      description: 'Close underlay',
      trigger: { device: 'keyboard', chord: { steps: [{ key: 'escape' }] } },
    }, 'test');

    engine.activateContext('overlay.underlay');

    const underlayEl = document.createElement('section');
    const menuEl = document.createElement('div');
    document.body.append(underlayEl, menuEl);

    const unregisterUnderlay = modalStack.register({
      id: 'underlay',
      element: underlayEl,
      contextId: 'overlay.underlay',
      dismiss: () => { underlayClosed = true; },
      dismissible: () => true,
    });
    const unregisterMenu = modalStack.register({
      id: 'top-menu',
      element: menuEl,
      dismiss: () => { menuClosed = true; },
      dismissible: () => true,
    });

    const escapeEvent = new KeyboardEvent('keydown', { key: 'Escape', cancelable: true, bubbles: true });
    window.dispatchEvent(escapeEvent);

    // InputEngine must yield because top-menu is above overlay.underlay
    expect(escapeEvent.defaultPrevented).toBeFalse();
    expect(underlayClosed).toBeFalse();

    // AppComponent global Escape handler calls modalStack.dismissTopmost()
    expect(modalStack.dismissTopmost()).toBeTrue();
    expect(menuClosed).toBeTrue();
    expect(underlayClosed).toBeFalse();

    unregisterMenu();
    const matchingEscape = new KeyboardEvent('keydown', { key: 'Escape', cancelable: true, bubbles: true });
    window.dispatchEvent(matchingEscape);
    expect(underlayClosed).toBeTrue();

    unregisterUnderlay();
    engine.deactivateContext('overlay.underlay');
    underlayEl.remove();
    menuEl.remove();
  });

  it('cancels InputCenter capture on Escape before closing the dialog', () => {
    const engineStub = {
      bindings$: { subscribe: () => () => undefined },
      formatInputTrigger: () => 'Click',
      setKeyboardEnabled: jasmine.createSpy('setKeyboardEnabled'),
    } as unknown as InputEngineService;
    const customizationStub = {
      bindings: () => [],
      conflicts: () => [],
      hasOverride: () => false,
    } as unknown as InputCustomizationService;
    const dialogsStub = {} as ConfirmDialogService;
    const cdrStub = { markForCheck: () => undefined } as ChangeDetectorRef;

    const component = new InputCenterComponent(
      customizationStub,
      engineStub,
      dialogsStub,
      cdrStub,
    );
    component.visible = true;

    let closedCount = 0;
    component.closed.subscribe(() => { closedCount += 1; });

    component.beginCapture({
      id: 'mouse.binding',
      commandId: 'test.cmd',
      contextId: 'global',
      source: 'default',
      description: 'Mouse binding',
      trigger: { device: 'mouse', gesture: { type: 'click', button: 'left' } },
    });

    expect(component.editingBinding).not.toBeNull();
    component.onPanelEscape(new KeyboardEvent('keydown', { key: 'Escape' }));
    expect(component.editingBinding).toBeNull();
    expect(closedCount).toBe(0);

    component.onPanelEscape(new KeyboardEvent('keydown', { key: 'Escape' }));
    expect(closedCount).toBe(1);
  });

  it('cancels WorkspaceSwitcher inline edit on Escape before closing', () => {
    const workspaceStub = {
      getWorkspaces: () => ({ workspaces: [] }),
      getActiveWorkspace: () => null,
    } as unknown as WorkspaceService;
    const surfaceStub = {
      active: null,
      listToolWorkspaces: () => [],
    } as unknown as WorkspaceSurfaceService;
    const confirmStub = {} as ConfirmDialogService;
    const inputStub = {
      activateContext: jasmine.createSpy('activateContext'),
      deactivateContext: jasmine.createSpy('deactivateContext'),
    } as unknown as InputEngineService;
    const cdrStub = {
      markForCheck: () => undefined,
      detectChanges: () => undefined,
    } as unknown as ChangeDetectorRef;

    const component = new WorkspaceSwitcherComponent(
      workspaceStub,
      surfaceStub,
      confirmStub,
      inputStub,
      cdrStub,
    );
    component.open();
    component.createWorkspace();
    expect(component.edit).not.toBeNull();

    component.onEscapeAction(new KeyboardEvent('keydown', { key: 'Escape' }));
    expect(component.edit).toBeNull();
    expect(component.isClosing).toBeFalse();

    component.onEscapeAction(new KeyboardEvent('keydown', { key: 'Escape' }));
    expect(component.isClosing).toBeTrue();
  });

  it('closes ContextMenuComponent on Escape even with empty items', () => {
    const cdrStub = { markForCheck: () => undefined } as ChangeDetectorRef;
    const menu = new ContextMenuComponent(cdrStub);
    menu.state = { x: 10, y: 10, items: [] };

    let closedCount = 0;
    menu.closed.subscribe(() => { closedCount += 1; });

    menu.onMenuKeyDown(new KeyboardEvent('keydown', { key: 'Escape' }));
    expect(closedCount).toBe(1);
  });

  it('closes FileContextMenuComponent on Escape with empty items and honors topmost stack', () => {
    const hostEl = document.createElement('div');
    const blockingEl = document.createElement('section');
    document.body.append(hostEl, blockingEl);

    TestBed.configureTestingModule({
      providers: [
        ModalStackService,
        { provide: ElementRef, useValue: new ElementRef(hostEl) },
      ],
    });

    const modalStack = TestBed.inject(ModalStackService);
    const menu = TestBed.runInInjectionContext(() => new FileContextMenuComponent());
    let closedCount = 0;
    menu.closed.subscribe(() => { closedCount += 1; });

    menu.state = { x: 0, y: 0, items: [] };
    menu.ngOnChanges({ state: { currentValue: menu.state, previousValue: null, firstChange: true, isFirstChange: () => true } });

    const unregisterBlocking = modalStack.register({
      id: 'blocking-modal',
      element: blockingEl,
      dismiss: () => undefined,
      dismissible: () => true,
    });

    menu.onEscape(new KeyboardEvent('keydown', { key: 'Escape' }));
    expect(closedCount).toBe(0);

    unregisterBlocking();
    menu.onMenuKeyDown(new KeyboardEvent('keydown', { key: 'Escape' }));
    expect(closedCount).toBe(1);

    menu.ngOnDestroy();
    hostEl.remove();
    blockingEl.remove();
  });

  it('registers open DropdownDirective instances in ModalStackService', () => {
    const hostEl = document.createElement('div');
    document.body.append(hostEl);

    TestBed.configureTestingModule({
      providers: [
        ModalStackService,
        DropdownService,
        { provide: ElementRef, useValue: new ElementRef(hostEl) },
      ],
    });

    const modalStack = TestBed.inject(ModalStackService);
    const dropdownService = TestBed.inject(DropdownService);
    const directive = TestBed.runInInjectionContext(
      () => new DropdownDirective(new ElementRef(hostEl), dropdownService),
    );
    directive.ngOnInit();

    let closedCount = 0;
    directive.closed.subscribe(() => { closedCount += 1; });

    directive.open();
    expect(directive.isOpen).toBeTrue();
    expect(modalStack.topmost()).not.toBeNull();

    expect(modalStack.dismissTopmost()).toBeTrue();
    expect(directive.isOpen).toBeFalse();
    expect(closedCount).toBe(1);
    expect(modalStack.topmost()).toBeNull();

    directive.ngOnDestroy();
    hostEl.remove();
  });
});
