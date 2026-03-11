export interface FileBrowserLayoutHost {
  getLeftPaneWidth(): number;
  getRightPaneWidth(): number;
  setPaneWidths(left: number, right: number): void;
}

export class FileBrowserLayoutController {
  private readonly minPaneWidth = 20;
  private readonly maxPaneWidth = 80;

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

    this.setPanePosition(host, parsed, false);
  }

  syncPaneWidths(host: FileBrowserLayoutHost, leftWidth: number): void {
    this.setPanePosition(host, leftWidth, true);
  }

  nudgeDividerLeft(host: FileBrowserLayoutHost): void {
    this.setPanePosition(host, host.getLeftPaneWidth() - 5, true);
  }

  nudgeDividerRight(host: FileBrowserLayoutHost): void {
    this.setPanePosition(host, host.getLeftPaneWidth() + 5, true);
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

  private setPanePosition(host: FileBrowserLayoutHost, leftWidth: number, persist: boolean): void {
    const left = this.clampWidth(leftWidth);
    host.setPaneWidths(left, 100 - left);
    if (persist) {
      this.savePaneWidths(host);
    }
  }
}