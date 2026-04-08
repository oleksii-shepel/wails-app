import { CommonModule } from '@angular/common';
import {
    Component,
    ElementRef,
    EventEmitter,
    HostBinding,
    HostListener,
    OnDestroy,
    Input,
    Output
} from '@angular/core';

@Component({
    selector: 'app-split-divider',
    standalone: true,
    template: `
    <div class="line"></div>

    <div class="handle"></div>

    <div class="tooltip" *ngIf="dragging">
    {{ snapPreview | number:'1.0-0' }}%
    </div>
  `,
    styles: [`
    :host {
        display: block;
        position: relative;
        z-index: var(--vsc-z-divider, 750);
        isolation: isolate;
        overflow: visible;
        user-select: none;
        color: var(--vsc-foreground, #cccccc);
    }

    :host.vertical {
        width: 10px;
        height: 100%;
        margin: 0 -4px;
        cursor: url("data:image/svg+xml,%3Csvg%20xmlns=%27http://www.w3.org/2000/svg%27%20width=%2730%27%20height=%2730%27%20viewBox=%270%200%2024%2024%27%3E%3Cpath%20fill=%27white%27%20d=%27M11%202h2v20h-2zM6.29%207.29l-4%204a.996.996%200%200%200%200%201.41l4%204%201.41-1.41L4.41%2012%207.7%208.71%206.29%207.3ZM17.71%207.29%2016.3%208.7l3.29%203.29-3.29%203.29%201.41%201.41%204-4a.996.996%200%200%200%200-1.41l-4-4Z%27/%3E%3C/svg%3E") 15 15, col-resize;
        transition: left .16s cubic-bezier(.2,.7,.3,1);
    }

    :host.horizontal {
        width: 100%;
        height: 10px;
        margin: -4px 0;
        cursor: row-resize;
        transition: top .16s cubic-bezier(.2,.7,.3,1);
    }

    /* center line - subtle border color */
    .line {
        position: absolute;
        inset: 0;
        margin: auto;
        z-index: 1;
        background: var(--vsc-border-subtle, #3a3a3a);
        box-shadow:
          0 0 0 1px color-mix(in srgb, var(--vsc-editor-background, #1e1e1e) 40%, transparent),
          0 0 8px color-mix(in srgb, var(--vsc-editor-background, #1e1e1e) 35%, transparent);
        transition: all var(--vsc-transition-fast, 0.1s ease);
    }

    :host.vertical .line {
        width: 2px;
        height: 100%;
    }

    :host.horizontal .line {
        width: 100%;
        height: 2px;
    }

    :host:hover .line,
    :host.hovering .line,
    :host-context(body.divider-hover) .line {
        background: var(--vsc-border-bright, #5a5a5a);
        box-shadow:
          0 0 0 1px color-mix(in srgb, var(--vsc-editor-background, #1e1e1e) 55%, transparent),
          0 0 10px color-mix(in srgb, var(--vsc-editor-background, #1e1e1e) 45%, transparent);
    }

    :host.vertical:hover .line,
    :host.vertical.hovering .line,
    :host-context(body.divider-hover).vertical .line {
        width: 2px;
    }

    :host.horizontal:hover .line,
    :host.horizontal.hovering .line,
    :host-context(body.divider-hover).horizontal .line {
        height: 2px;
    }

    :host.dragging .line,
    :host-context(body.divider-dragging) .line {
        background: var(--vsc-accent-blue, #007acc);
        box-shadow: 0 0 0 2px color-mix(in srgb, var(--vsc-accent-blue, #007acc) 20%, transparent);
    }

    :host.vertical.dragging .line,
    :host-context(body.divider-dragging).vertical .line {
        width: 2px;
    }

    :host.horizontal.dragging .line,
    :host-context(body.divider-dragging).horizontal .line {
        height: 2px;
    }

    :host.dragging {
        transition: none;
    }

    /* center pill handle - light grey, unobtrusive */
    .handle {
        position: absolute;
        left: 50%;
        top: 50%;
        transform: translate(-50%, -50%);

        border-radius: var(--vsc-border-radius-lg, 4px);
        z-index: 2;

        /* Light grey, subtle background */
        background: var(--vsc-foreground-dim, #888888);
        border: 1px solid var(--vsc-border-subtle, #3a3a3a);

        opacity: 0;
        transition: all var(--vsc-transition-fast, 0.1s ease);
        box-shadow:
          0 6px 12px color-mix(in srgb, black 30%, transparent),
          0 0 0 1px color-mix(in srgb, var(--vsc-editor-background, #1e1e1e) 55%, transparent);
    }

    :host.vertical .handle {
        width: 8px;
        height: 28px;
    }

    :host.horizontal .handle {
        width: 28px;
        height: 8px;
    }

    /* hover = slightly more visible but still subtle */
    :host:hover .handle,
    :host.hovering .handle,
    :host-context(body.divider-hover) .handle {
        opacity: 0.7;
        background: var(--vsc-foreground, #cccccc);
        border-color: var(--vsc-border, #454545);
        transform: translate(-50%, -50%) scale(1.05);
    }

    /* dragging = accent border, still grey fill */
    :host.dragging .handle,
    :host-context(body.divider-dragging) .handle {
        opacity: 1;
        background: var(--vsc-foreground, #cccccc);
        border-color: var(--vsc-accent-blue, #007acc);
        box-shadow: 0 0 0 2px color-mix(in srgb, var(--vsc-accent-blue, #007acc) 25%, transparent);
        transform: translate(-50%, -50%) scale(1.05);
    }

    /* show on hover or drag */
    :host:hover .handle,
    :host.hovering .handle,
    :host.dragging .handle,
    :host-context(body.divider-hover) .handle,
    :host-context(body.divider-dragging) .handle {
        opacity: 1;
    }

    /* tooltip - VS Code style */
    .tooltip {
        position: absolute;

        padding: 4px 8px;
        font-size: var(--vsc-font-size-small, 12px);
        font-family: var(--vsc-font-family-mono, monospace);
        font-weight: 400;

        background: var(--vsc-editor-background, #1e1e1e);
        border: 1px solid var(--vsc-border, #454545);
        border-radius: var(--vsc-border-radius, 3px);

        color: var(--vsc-foreground, #cccccc);
        white-space: nowrap;
        pointer-events: none;
        z-index: 3;

        box-shadow: var(--vsc-widget-shadow, 0 5px 8px 0 rgba(0, 0, 0, 0.36));

        opacity: 0;
        transition: opacity var(--vsc-transition-fast, 0.1s ease), transform var(--vsc-transition-fast, 0.1s ease);
    }

    :host.vertical .tooltip {
        top: 50%;
        left: 100%;
        transform: translate(8px, -50%);
    }

    :host.horizontal .tooltip {
        top: 100%;
        left: 50%;
        transform: translate(-50%, 8px);
    }

    :host.vertical.dragging .tooltip {
        opacity: 1;
        transform: translate(12px, -50%);
    }

    :host.horizontal.dragging .tooltip {
        opacity: 1;
        transform: translate(-50%, 12px);
    }

    /* Light theme adjustments - using vscode-light class */
    :host-context(.vscode-light) .handle {
        background: var(--vsc-foreground-dim, #999999);
        border-color: var(--vsc-border-subtle, #e5e5e5);
    }

    :host-context(.vscode-light):hover .handle {
        background: var(--vsc-foreground, #616161);
        border-color: var(--vsc-border, #d4d4d4);
    }

    :host-context(.vscode-light).dragging .handle {
        background: var(--vsc-foreground, #616161);
        border-color: var(--vsc-accent-blue, #007acc);
    }

    :host-context(.vscode-light) .tooltip {
        background: var(--vsc-editor-background, #ffffff);
        border-color: var(--vsc-border, #d4d4d4);
        color: var(--vsc-foreground, #616161);
    }

    /* Compact mode adjustments */
    :host-context(.compact-mode) .handle {
        width: 8px;
        height: 24px;
    }

    :host-context(.compact-mode).horizontal .handle {
        width: 24px;
        height: 8px;
    }

    :host-context(.compact-mode) .tooltip {
        padding: 2px 6px;
        font-size: 11px;
    }

    /* Ensure the pill stays grey in all states */
    :host .handle {
        background: var(--vsc-foreground-dim, #888888) !important;
    }

    :host:hover .handle {
        background: var(--vsc-foreground, #cccccc) !important;
    }

    :host.dragging .handle {
        background: var(--vsc-foreground, #cccccc) !important;
    }

    :host-context(.vscode-light) .handle {
        background: var(--vsc-foreground-dim, #999999) !important;
    }

    :host-context(.vscode-light):hover .handle {
        background: var(--vsc-foreground, #616161) !important;
    }

    :host-context(.vscode-light).dragging .handle {
        background: var(--vsc-foreground, #616161) !important;
    }
    `],
    imports: [CommonModule]
})
export class SplitDividerComponent implements OnDestroy {
    private readonly hoverEnterDelayMs = 0;
    private readonly hoverLeaveDelayMs = 120;
    private readonly hoverOutsetPx = 12;
    private hoverEnterTimer: ReturnType<typeof setTimeout> | null = null;
    private hoverLeaveTimer: ReturnType<typeof setTimeout> | null = null;
    private lastPointerX = 0;
    private lastPointerY = 0;
    /* ---------------- INPUTS ---------------- */

    @Input() position = 50;
    @Output() positionChange = new EventEmitter<number>();

    @Input() orientation: 'vertical' | 'horizontal' = 'vertical';

    @Input() min = 20;
    @Input() max = 80;

    @Input() snapPoints: number[] = [20, 25, 33.333, 50, 66.666, 75, 80];

    /* ---------------- STATE ---------------- */

    dragging = false;
    hovering = false;
    snapPreview = 50;
    private startPointerPosition = 0;
    private startPos = 50;

    /** Base snap threshold in %. Actual threshold per point is scaled by proximity to center. */
    readonly SNAP_THRESHOLD_MIN = 2;  // threshold at the edges (20%, 80%)
    readonly SNAP_THRESHOLD_MAX = 8;  // threshold at dead center (50%)

    constructor(private el: ElementRef<HTMLElement>) { }

    ngOnDestroy(): void {
        this.clearHoverTimers();
        this.hovering = false;
        this.setBodyInteractionClass('divider-hover', false);
        this.setBodyInteractionClass('divider-dragging', false);
    }

    /* ---------------- DRAG ---------------- */

    @HostListener('pointerdown', ['$event'])
    startDrag(e: PointerEvent) {
        e.preventDefault();

        this.clearHoverTimers();
        this.dragging = true;
        this.hovering = true;
        this.startPointerPosition = this.isHorizontal ? e.clientY : e.clientX;
        this.startPos = this.position;

        this.setBodyInteractionClass('divider-hover', true);
        this.setBodyInteractionClass('divider-dragging', true);
    }

    @HostListener('pointerenter', ['$event'])
    onPointerEnter(event: PointerEvent) {
        this.lastPointerX = event.clientX;
        this.lastPointerY = event.clientY;
        if (this.hoverLeaveTimer !== null) {
            clearTimeout(this.hoverLeaveTimer);
            this.hoverLeaveTimer = null;
        }
        if (this.dragging || document.body.classList.contains('divider-hover')) {
            this.activateHover();
            return;
        }
        if (this.hoverEnterTimer !== null) {
            return;
        }
        this.hoverEnterTimer = setTimeout(() => {
            this.hoverEnterTimer = null;
            this.activateHover();
        }, this.hoverEnterDelayMs);
    }

    @HostListener('pointerleave', ['$event'])
    onPointerLeave(event?: PointerEvent) {
        if (event) {
            this.lastPointerX = event.clientX;
            this.lastPointerY = event.clientY;
        }
        if (this.hoverEnterTimer !== null) {
            clearTimeout(this.hoverEnterTimer);
            this.hoverEnterTimer = null;
        }
        if (this.dragging) {
            return;
        }
        if (this.isPointerWithinHoverZone(this.lastPointerX, this.lastPointerY)) {
            return;
        }
        if (this.hoverLeaveTimer !== null) {
            return;
        }
        this.hoverLeaveTimer = setTimeout(() => {
            this.hoverLeaveTimer = null;
            if (!this.el.nativeElement.matches(':hover') && !this.isPointerWithinHoverZone(this.lastPointerX, this.lastPointerY)) {
                this.deactivateHover();
            }
        }, this.hoverLeaveDelayMs);
    }

    @HostListener('document:pointermove', ['$event'])
    onMove(e: PointerEvent) {
        this.lastPointerX = e.clientX;
        this.lastPointerY = e.clientY;

        if (!this.dragging) {
            if (this.isPointerWithinHoverZone(e.clientX, e.clientY)) {
                if (this.hoverLeaveTimer !== null) {
                    clearTimeout(this.hoverLeaveTimer);
                    this.hoverLeaveTimer = null;
                }
                if (this.hovering) {
                    this.setBodyInteractionClass('divider-hover', true);
                }
            } else if (this.hovering && !this.el.nativeElement.matches(':hover')) {
                this.onPointerLeave(e);
            }
            return;
        }

        const container = this.el.nativeElement.parentElement!;
        const rect = container.getBoundingClientRect();
        const axisSize = this.isHorizontal ? rect.height : rect.width;
        if (axisSize <= 0) {
            return;
        }

        const pointerPosition = this.isHorizontal ? e.clientY : e.clientX;
        const delta = pointerPosition - this.startPointerPosition;
        const percent = Math.max(
            this.min,
            Math.min(this.max, this.startPos + (delta / axisSize) * 100),
        );

        // Raw 1:1 tracking during drag — no magnet, no pull.
        // Snapping only happens on release so the divider follows the cursor exactly.
        this.position = percent;
        this.snapPreview = this.snapToNearest(percent);
        this.positionChange.emit(percent);
        window.dispatchEvent(new CustomEvent('file-manager-divider-move'));
    }

    @HostListener('document:pointerup')
    stopDrag() {
        if (!this.dragging) return;

        this.dragging = false;
        this.setBodyInteractionClass('divider-dragging', false);
        if (this.el.nativeElement.matches(':hover')) {
            this.activateHover();
        } else {
            this.onPointerLeave();
        }

        const final = this.snapToNearest(this.position);
        this.position = final;
        this.positionChange.emit(final);
    }

    private clearHoverTimers(): void {
        if (this.hoverEnterTimer !== null) {
            clearTimeout(this.hoverEnterTimer);
            this.hoverEnterTimer = null;
        }
        if (this.hoverLeaveTimer !== null) {
            clearTimeout(this.hoverLeaveTimer);
            this.hoverLeaveTimer = null;
        }
    }

    private activateHover(): void {
        this.hovering = true;
        this.setBodyInteractionClass('divider-hover', true);
    }

    private deactivateHover(): void {
        this.hovering = false;
        this.setBodyInteractionClass('divider-hover', false);
    }

    private isPointerWithinHoverZone(clientX: number, clientY: number): boolean {
        const rect = this.el.nativeElement.getBoundingClientRect();
        if (this.isHorizontal) {
            return clientX >= rect.left
                && clientX <= rect.right
                && clientY >= rect.top - this.hoverOutsetPx
                && clientY <= rect.bottom + this.hoverOutsetPx;
        }

        return clientX >= rect.left - this.hoverOutsetPx
            && clientX <= rect.right + this.hoverOutsetPx
            && clientY >= rect.top
            && clientY <= rect.bottom;
    }

    private setBodyInteractionClass(className: string, active: boolean): void {
        const classList = document.body.classList;
        const changed = active ? !classList.contains(className) : classList.contains(className);
        classList.toggle(className, active);
        if (changed) {
            window.dispatchEvent(new CustomEvent('file-manager-divider-statechange'));
        }
    }

    /* ---------------- DOUBLE CLICK RESET ---------------- */

    @HostListener('dblclick')
    reset() {
        this.position = 50;
        this.positionChange.emit(50);
    }

    /* ---------------- SNAP ENGINE ---------------- */

    /**
     * Returns the nearest snap point if the release falls within that point's
     * threshold, otherwise returns the value unchanged.
     *
     * Each snap point gets its own threshold that scales linearly with how
     * close the point is to the center (50%): center snaps are wider/more
     * magnetic, edge snaps are narrower/more precise.
     */
    private snapToNearest(value: number): number {
        let bestCandidate = value;
        let bestMargin = 0; // how far inside the threshold we are (higher = better)

        for (const p of this.snapPoints) {
            const threshold = this.thresholdForPoint(p);
            const dist = Math.abs(p - value);
            const margin = threshold - dist; // positive means we're inside the zone

            if (margin > 0 && margin > bestMargin) {
                bestMargin = margin;
                bestCandidate = p;
            }
        }

        return bestCandidate;
    }

    /**
     * Threshold for a given snap point, interpolated between MIN (at the
     * allowed edges) and MAX (at 50% center). Uses a smooth curve so the
     * transition doesn't feel abrupt.
     */
    private thresholdForPoint(point: number): number {
        const center = 50;
        const edgeDist = Math.max(Math.abs(this.min - center), Math.abs(this.max - center));
        const distFromCenter = Math.abs(point - center);
        // t = 0 at center, 1 at edge
        const t = Math.min(distFromCenter / edgeDist, 1);
        // ease-out curve so center zone feels noticeably wider
        const eased = t * t;
        return this.SNAP_THRESHOLD_MAX - eased * (this.SNAP_THRESHOLD_MAX - this.SNAP_THRESHOLD_MIN);
    }

    /* ---------------- CLASS ---------------- */

    @HostBinding('class.dragging')
    get isDragging() {
        return this.dragging;
    }

    @HostBinding('class.hovering')
    get isHovering() {
        return this.hovering;
    }

    @HostBinding('class.horizontal')
    get isHorizontal(): boolean {
        return this.orientation === 'horizontal';
    }

    @HostBinding('class.vertical')
    get isVertical(): boolean {
        return !this.isHorizontal;
    }
}
