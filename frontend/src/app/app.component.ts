import { AfterViewInit, Component, HostListener, OnDestroy, OnInit, ViewChild } from '@angular/core';
import { FileBrowserComponent } from './components/file-browser.component';
import { ConfirmDialogComponent } from './components/confirm-dialog.component';
import { RemoteConnectDialogComponent } from './components/remote-connect-dialog.component';
import { SettingsDialogComponent } from './components/settings-dialog.component';
import { BatchRenameDialogComponent } from './components/batch-rename-dialog.component';
import { SettingsService } from './services/settings.service';
import { ThemeService } from './services/theme.service';
import { OperationService } from './services/operation.service';
import { PluginBootstrapService } from './services/plugin-bootstrap.service';

@Component({
  selector: 'app-root',
  standalone: true,
  imports: [FileBrowserComponent, ConfirmDialogComponent, RemoteConnectDialogComponent, SettingsDialogComponent, BatchRenameDialogComponent],
  template: `
    <app-file-browser></app-file-browser>
    <app-confirm-dialog></app-confirm-dialog>
    <app-remote-connect-dialog></app-remote-connect-dialog>
    <app-settings-dialog></app-settings-dialog>
    <app-batch-rename-dialog></app-batch-rename-dialog>
  `,
  styles: [`
    :host {
      display: block;
      width: 100vw;
      height: 100vh;
      overflow: hidden;
    }
  `]
})
export class AppComponent implements OnInit, AfterViewInit, OnDestroy {
  @ViewChild(FileBrowserComponent) private fileBrowser?: FileBrowserComponent;

  title = 'File Manager';
  private settingsSubscription: { unsubscribe(): void } | null = null;

  constructor(
    public theme: ThemeService,
    public operationService: OperationService,
    private readonly settingsService: SettingsService,
    private readonly pluginBootstrap: PluginBootstrapService,
  ) {}

  private captureDragGuard = (event: DragEvent): void => {
    if (this.isExternalFileDrag(event)) {
      // For dragenter/dragover: prevent default to enable drop
      // For drop: DON'T prevent default - let Wails handle it
      if (event.type === 'dragenter' || event.type === 'dragover') {
        event.preventDefault();
        if (event.dataTransfer) {
          event.dataTransfer.dropEffect = 'copy';
        }
      }
      // Let drop event propagate - Wails will intercept it
    }
  };

  ngOnInit(): void {
    // Wails' OnFileDrop() registers its own dragover/drop handlers - no custom listeners needed
    this.settingsSubscription = this.settingsService.getSettings$().subscribe((settings) => {
      this.theme.setThemePreference(settings.general.theme);
      this.theme.setCompact(settings.view.compactMode);
      this.applyAccentColor(settings.general.accentColor);
      this.applyUiZoom(settings.general.uiZoom);
    });
  }

  ngAfterViewInit(): void {
    // Plugin discovery/activation starts only after the root view and Wails UI
    // boundary exist. Failures are isolated inside PluginBootstrapService and
    // never block normal application startup.
    void this.pluginBootstrap.initialize(
      () => this.fileBrowser?.getActiveSelectionPaths() ?? [],
    );
  }

  ngOnDestroy(): void {
    this.settingsSubscription?.unsubscribe();
    void this.pluginBootstrap.shutdown();

    window.removeEventListener('dragenter', this.captureDragGuard, true);
    window.removeEventListener('dragover', this.captureDragGuard, true);

    document.removeEventListener('dragenter', this.captureDragGuard, true);
    document.removeEventListener('dragover', this.captureDragGuard, true);
  }

  @HostListener('window:dragover', ['$event'])
  onWindowDragOver(event: DragEvent): void {
    if (this.isExternalFileDrag(event)) {
      event.preventDefault();
      if (event.dataTransfer) {
        event.dataTransfer.dropEffect = event.ctrlKey || event.metaKey ? 'copy' : 'move';
      }
    }
  }

  // Removed window:drop HostListener - let Wails handle drops natively

  private isExternalFileDrag(event: DragEvent): boolean {
    const types = event.dataTransfer?.types;
    if (!types) return false;
    if (types.includes('application/json')) return false;
    return types.includes('Files') || (event.dataTransfer?.files?.length ?? 0) > 0;
  }

  private applyAccentColor(rawAccentColor: string): void {
    const root = document.documentElement;
    const accentColor = this.normalizeAccentColor(rawAccentColor);
    const accentProperties = [
      '--vsc-accent-blue',
      '--vsc-accent',
      '--vsc-input-focus-border',
      '--vsc-statusbar-background',
      '--vsc-list-active-background',
      '--vsc-list-focus-background',
    ];

    if (!accentColor) {
      for (const property of accentProperties) {
        root.style.removeProperty(property);
      }
      return;
    }

    root.style.setProperty('--vsc-accent-blue', accentColor);
    root.style.setProperty('--vsc-accent', accentColor);
    root.style.setProperty('--vsc-input-focus-border', accentColor);
    root.style.setProperty('--vsc-statusbar-background', accentColor);
    root.style.setProperty('--vsc-list-active-background', `color-mix(in srgb, ${accentColor} 32%, transparent)`);
    root.style.setProperty('--vsc-list-focus-background', `color-mix(in srgb, ${accentColor} 24%, transparent)`);
  }

  private applyUiZoom(rawZoom: number): void {
    const zoom = Number.isFinite(rawZoom) ? Math.min(150, Math.max(75, rawZoom)) : 100;
    const root = document.documentElement;

    if (zoom === 100) {
      root.style.removeProperty('zoom');
      return;
    }

    root.style.setProperty('zoom', String(zoom / 100));
  }

  private normalizeAccentColor(rawAccentColor: string): string | null {
    const value = rawAccentColor.trim();
    if (!value) return null;
    return /^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6})$/.test(value) ? value : null;
  }
}