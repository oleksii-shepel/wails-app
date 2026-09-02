export type SelectionBasketOperation = 'copy' | 'move' | 'delete';
export type SelectionBasketDestination = 'left' | 'right';

export interface SelectionBasketOperationRequest {
  operation: SelectionBasketOperation;
  /**
   * Pane destination retained for the fast left/right path.
   * When destinationPath is present it takes precedence.
   */
  destinationSide: SelectionBasketDestination;
  /** Explicit folder selected with the native directory picker. */
  destinationPath?: string;
  recursive: boolean;
  useTrash: boolean;
}
