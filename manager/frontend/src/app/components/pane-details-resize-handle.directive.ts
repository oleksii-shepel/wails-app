import {
  AfterViewInit,
  Directive,
  ElementRef,
  EventEmitter,
  HostBinding,
  HostListener,
  Input,
  OnDestroy,
  Output,
} from '@angular/core';

export interface PaneDetailsColumnWidthChange {
  readonly key: string;
  readonly width: number;
}

/**
 * The single column-resize interaction used by every Details surface.
 *
 * The containing Details root must expose `data-pane-details-root`. During a
 * drag we mutate one inherited CSS custom property, so header and visible rows
 * resize together without rebuilding row models or running per-row work.
 */
@Directive({
  selector: '[paneDetailsResizeHandle]',
  standalone: true,
})
export class PaneDetailsResizeHandleDirective implements AfterViewInit, OnDestroy {
  @Input('paneDetailsResizeHandle') key = '';
  @Input() paneDetailsMinWidth = 80;
  @Input() paneDetailsMaxWidth = 1200;

  @Output() paneDetailsWidthCommit = new EventEmitter<PaneDetailsColumnWidthChange>();

  @HostBinding('class.pane-details-resize-handle') readonly resizeHandleClass = true;
  @HostBinding('style.position') readonly position = 'absolute';
  @HostBinding('style.top') readonly top = '0';
  @HostBinding('style.bottom') readonly bottom = '0';
  @HostBinding('style.right') readonly right = '-4px';
  @HostBinding('style.width') readonly width = '8px';
  @HostBinding('style.cursor') readonly cursor = 'col-resize';
  @HostBinding('style.z-index') readonly zIndex = '6';
  @HostBinding('style.user-select') readonly userSelect = 'none';
  @HostBinding('style.background') readonly delimiterBackground =
    'linear-gradient(to right, transparent calc(50% - 0.5px), var(--vsc-border) calc(50% - 0.5px), var(--vsc-border) calc(50% + 0.5px), transparent calc(50% + 0.5px))';
  @HostBinding('style.opacity') delimiterOpacity = '0.72';
  @HostBinding('class.pane-details-resize-handle--dragging') dragging = false;

  private root: HTMLElement | null = null;
  private startX = 0;
  private startWidth = 0;
  private currentWidth = 0;
  private moveListener?: (event: MouseEvent) => void;
  private upListener?: () => void;

  constructor(private readonly elementRef: ElementRef<HTMLElement>) {}

  ngAfterViewInit(): void {
    const parent = this.elementRef.nativeElement.parentElement;
    if (parent && getComputedStyle(parent).position === 'static') {
      parent.style.position = 'relative';
    }
  }

  @HostListener('mouseenter')
  onMouseEnter(): void {
    this.delimiterOpacity = '1';
  }

  @HostListener('mouseleave')
  onMouseLeave(): void {
    if (!this.moveListener) this.delimiterOpacity = '0.72';
  }

  @HostListener('mousedown', ['$event'])
  onMouseDown(event: MouseEvent): void {
    if (event.button !== 0 || !this.key) return;

    const handle = this.elementRef.nativeElement;
    const cell = handle.parentElement;
    const root = handle.closest<HTMLElement>('[data-pane-details-root]');
    if (!cell || !root) return;

    event.preventDefault();
    event.stopPropagation();

    this.cleanup();
    this.root = root;
    this.dragging = true;
    root.setAttribute('data-pane-column-resizing', this.key);
    this.startX = event.clientX;
    this.startWidth = Math.round(cell.getBoundingClientRect().width);
    this.currentWidth = this.clamp(this.startWidth);
    this.applyWidth(this.currentWidth);

    this.moveListener = moveEvent => {
      this.currentWidth = this.clamp(this.startWidth + moveEvent.clientX - this.startX);
      this.applyWidth(this.currentWidth);
    };
    this.upListener = () => {
      const width = this.currentWidth;
      this.cleanup();
      document.body.style.cursor = '';
      document.body.style.userSelect = '';
      this.delimiterOpacity = '0.72';
      this.paneDetailsWidthCommit.emit({ key: this.key, width });
    };

    window.addEventListener('mousemove', this.moveListener);
    window.addEventListener('mouseup', this.upListener);
    document.body.style.cursor = 'col-resize';
    document.body.style.userSelect = 'none';
    this.delimiterOpacity = '1';
  }

  ngOnDestroy(): void {
    this.cleanup();
    document.body.style.cursor = '';
    document.body.style.userSelect = '';
  }

  private applyWidth(width: number): void {
    this.root?.style.setProperty(this.variableName(this.key), `${width}px`);
  }

  private clamp(width: number): number {
    return Math.max(
      Math.max(1, this.paneDetailsMinWidth),
      Math.min(Math.max(this.paneDetailsMinWidth, this.paneDetailsMaxWidth), Math.round(width)),
    );
  }

  private variableName(key: string): string {
    return `--details-column-${key.replace(/[^a-zA-Z0-9_-]/g, '-')}-width`;
  }

  private cleanup(): void {
    if (this.moveListener) {
      window.removeEventListener('mousemove', this.moveListener);
      this.moveListener = undefined;
    }
    if (this.upListener) {
      window.removeEventListener('mouseup', this.upListener);
      this.upListener = undefined;
    }
    this.root?.removeAttribute('data-pane-column-resizing');
    this.dragging = false;
    this.root = null;
  }
}