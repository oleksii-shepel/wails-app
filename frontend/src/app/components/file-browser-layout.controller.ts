export interface FileBrowserLayoutHost {
  getPanesContainer(): HTMLElement | undefined;
  getLeftPaneWidth(): number;
  getRightPaneWidth(): number;
  setPaneWidths(left: number, right: number): void;
  getIsResizing(): boolean;
  setIsResizing(resizing: boolean): void;
}

export class FileBrowserLayoutController {
  private readonly minPaneWidth = 20;
  private readonly maxPaneWidth = 80;
  private startX = 0;
  private startLeftWidth = 0;
  private activeHost: FileBrowserLayoutHost | null = null;

  private readonly boundMouseMove = (event: MouseEvent) => {
    if (this.activeHost) {
      this.onResize(this.activeHost, event);
    }
  };

  private readonly boundMouseUp = () => {
    if (this.activeHost) {
      this.stopResize(this.activeHost);
    }
  };

  loadPaneWidths(host: FileBrowserLayoutHost): void {
    const savedSplit = localStorage.getItem('split-pos');
    const savedLeftWidth = localStorage.getItem('fileManager_leftPaneWidth');
    const raw = savedSplit ?? savedLeftWidth;
    if (!raw) {
      return;
    }

    const parsed = parseFloat(raw);
    if (!Number.isFinite(parsed)) {
      return;
    }

    const clampedLeft = this.clampWidth(parsed);
    host.setPaneWidths(clampedLeft, 100 - clampedLeft);
  }

  startResize(host: FileBrowserLayoutHost, event: MouseEvent): void {
    event.preventDefault();
    this.activeHost = host;
    host.setIsResizing(true);
    this.startX = event.clientX;
    this.startLeftWidth = host.getLeftPaneWidth();

    document.addEventListener('mousemove', this.boundMouseMove);
    document.addEventListener('mouseup', this.boundMouseUp);
    document.body.classList.add('resizing');
  }

  resetPaneWidths(host: FileBrowserLayoutHost): void {
    host.setPaneWidths(50, 50);
    this.savePaneWidths(host);
  }

  cleanupResizeListeners(host: FileBrowserLayoutHost): void {
    if (!host.getIsResizing()) {
      return;
    }

    document.removeEventListener('mousemove', this.boundMouseMove);
    document.removeEventListener('mouseup', this.boundMouseUp);
    document.body.classList.remove('resizing');
    host.setIsResizing(false);
    this.activeHost = null;
  }

  nudgeDividerLeft(host: FileBrowserLayoutHost): void {
    const left = this.clampWidth(host.getLeftPaneWidth() - 5);
    host.setPaneWidths(left, 100 - left);
    this.savePaneWidths(host);
  }

  nudgeDividerRight(host: FileBrowserLayoutHost): void {
    const left = this.clampWidth(host.getLeftPaneWidth() + 5);
    host.setPaneWidths(left, 100 - left);
    this.savePaneWidths(host);
  }

  private onResize(host: FileBrowserLayoutHost, event: MouseEvent): void {
    if (!host.getIsResizing()) {
      return;
    }

    const container = host.getPanesContainer();
    if (!container) {
      return;
    }

    event.preventDefault();
    const deltaX = event.clientX - this.startX;
    const deltaPercent = (deltaX / container.clientWidth) * 100;
    const left = this.clampWidth(this.startLeftWidth + deltaPercent);
    host.setPaneWidths(left, 100 - left);
  }

  private stopResize(host: FileBrowserLayoutHost): void {
    if (!host.getIsResizing()) {
      return;
    }

    host.setIsResizing(false);
    document.removeEventListener('mousemove', this.boundMouseMove);
    document.removeEventListener('mouseup', this.boundMouseUp);
    document.body.classList.remove('resizing');
    this.activeHost = null;
    this.savePaneWidths(host);
  }

  private savePaneWidths(host: FileBrowserLayoutHost): void {
    const left = host.getLeftPaneWidth();
    const right = host.getRightPaneWidth();
    localStorage.setItem('fileManager_leftPaneWidth', left.toString());
    localStorage.setItem('fileManager_rightPaneWidth', right.toString());
    localStorage.setItem('split-pos', left.toString());
  }

  private clampWidth(width: number): number {
    return Math.max(this.minPaneWidth, Math.min(this.maxPaneWidth, width));
  }
}