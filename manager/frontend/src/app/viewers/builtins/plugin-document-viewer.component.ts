import { CommonModule } from '@angular/common';
import { ChangeDetectionStrategy, ChangeDetectorRef, Component, ElementRef, Input, OnChanges, OnDestroy, ViewChild, inject } from '@angular/core';
import type { FileProbe } from '../../shared/protocol';
import type { ViewerFileListItem } from '../../shared/protocol-enhanced';
import { ThemeService } from '../../services/theme.service';
import { DocumentRendererRegistryService } from '../document-renderer-registry.service';
import type { ViewerArchiveContext } from '../viewer-archive-context';
import { withViewerFrameScrollbars } from '../viewer-frame-scrollbars';

@Component({
  selector: 'app-plugin-document-viewer',
  standalone: true,
  imports: [CommonModule],
  changeDetection: ChangeDetectionStrategy.OnPush,
  template: `
    <div class="viewer-state" *ngIf="loading">Rendering document…</div>
    <div class="viewer-state error" *ngIf="error" role="alert">{{ error }}</div>
    <iframe #frame *ngIf="html && !loading" class="document" sandbox="" title="Document preview"></iframe>
  `,
  styles: [`:host { display:flex; flex:1 1 0%; min-height:0; } .document { border:0; flex:1 1 0%; min-height:0; width:100%; background:var(--vsc-document-canvas-background); } .viewer-state { display:grid; place-items:center; flex:1; color:var(--vsc-foreground); } .error { color:var(--vsc-error-foreground); padding:20px; text-align:center; }`],
})
export class PluginDocumentViewerComponent implements OnChanges, OnDestroy {
  @Input() targetPath = '';
  @Input() fileList: ViewerFileListItem[] = [];
  @Input() archiveContext: ViewerArchiveContext | null = null;
  @Input() viewerId: string | null = null;
  @Input() probe: FileProbe | null = null;
  @ViewChild('frame') frame?: ElementRef<HTMLIFrameElement>;

  html = '';
  loading = false;
  error = '';
  private requestId = 0;
  private renderedDocument = '';
  private readonly renderers = inject(DocumentRendererRegistryService);
  private readonly cdr = inject(ChangeDetectorRef);
  private readonly themeService = inject(ThemeService);
  private readonly themeSubscription = this.themeService.getTheme$().subscribe(() => {
    if (!this.renderedDocument) return;
    this.html = withViewerFrameScrollbars(this.renderedDocument);
    if (this.frame) this.frame.nativeElement.srcdoc = this.html;
    this.cdr.markForCheck();
  });

  ngOnChanges(): void {
    const requestId = ++this.requestId;
    const request = { targetPath: this.targetPath, fileList: this.fileList, archiveContext: this.archiveContext, viewerId: this.viewerId, probe: this.probe };
    const renderer = this.renderers.resolve(request);
    if (!renderer) {
      this.html = '';
      this.renderedDocument = '';
      this.loading = false;
      this.error = 'No installed plugin can render this document.';
      this.cdr.markForCheck();
      return;
    }
    this.loading = true;
    this.html = '';
    this.renderedDocument = '';
    this.error = '';
    this.cdr.markForCheck();
    renderer.render(request).then(result => {
      if (requestId !== this.requestId) return;
      this.renderedDocument = result.html;
      this.html = withViewerFrameScrollbars(this.renderedDocument);
      this.loading = false;
      this.cdr.detectChanges();
      if (this.frame) this.frame.nativeElement.srcdoc = this.html;
    }).catch(error => {
      if (requestId !== this.requestId) return;
      this.error = error instanceof Error ? error.message : String(error);
      this.loading = false;
      this.cdr.markForCheck();
    });
  }

  ngOnDestroy(): void {
    this.themeSubscription.unsubscribe();
  }
}
