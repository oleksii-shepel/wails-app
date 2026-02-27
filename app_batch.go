package main

// ============================================================================
// Batch Operations — exposed to the Wails frontend
// ============================================================================
//
// These methods wrap the batch_operations.go implementation and are the only
// entry points the frontend should call for multi-item operations.

// BatchCopy copies a list of source→destination pairs, tracking progress in
// real time.  Conflicts are surfaced as "operation:conflict" events; the
// frontend must call ResolveConflict to unblock each one.
func (a *App) BatchCopy(req BatchRequest) (*BatchResult, error) {
	return a.runBatchCopy(req)
}

// BatchMove moves a list of source→destination pairs.  Like BatchCopy,
// conflicts pause the operation until ResolveConflict is called.
func (a *App) BatchMove(req BatchRequest) (*BatchResult, error) {
	return a.BatchMoveWithProgress(req)
}

// BatchDelete deletes a list of paths, tracking progress in real time.
func (a *App) BatchDelete(req BatchDeleteRequest) (*BatchResult, error) {
	return a.BatchDeleteWithProgress(req)
}

// ResolveConflict unblocks a paused copy/move operation by supplying the
// user's decision for a specific conflict.
//
// conflictID matches the ConflictEvent.ConflictID that was emitted on the
// "operation:conflict" channel.  The resolution field must be one of:
//   "skip"      – leave destination untouched, continue with remaining items
//   "overwrite" – replace the destination with the source
//   "rename"    – copy/move source to a unique auto-generated name
//   "cancel"    – abort the entire batch immediately
//
// applyToAll, when true, uses the same resolution for all subsequent
// conflicts in this operation without showing further dialogs.
//
// Returns false if the conflictID is not recognised (e.g. already resolved or
// the operation was already cancelled).
func (a *App) ResolveConflict(conflictID string, resolution ConflictResolution, applyToAll bool) bool {
	return globalConflicts.resolve(conflictID, ConflictResolutionRequest{
		Resolution: resolution,
		ApplyToAll: applyToAll,
	})
}
