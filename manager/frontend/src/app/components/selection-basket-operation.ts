export type SelectionBasketOperation = 'copy' | 'move' | 'delete';
export type SelectionBasketDestination = 'left' | 'right';

export interface SelectionBasketOperationRequest {
  operation: SelectionBasketOperation;
  destinationSide: SelectionBasketDestination;
  recursive: boolean;
  useTrash: boolean;
}
