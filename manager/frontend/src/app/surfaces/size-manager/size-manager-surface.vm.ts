export interface SizeManagerSurfaceVm {
  paneId: 'left' | 'right';
  /** Root path being analysed. */
  analysisPath: string;

  /** Called when the user wants to navigate into a subfolder within the analyser. */
  onAnalyse(path: string): void;
  /** Called when the user wants to close the surface and go back to dir browsing. */
  onClose(): void;
}