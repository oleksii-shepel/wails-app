package contracts

import core "github.com/epikodelabs/reassembly/core/contracts"

type OperationType = core.OperationType

const (
	OpCopy      = core.OpCopy
	OpMove      = core.OpMove
	OpDelete    = core.OpDelete
	OpArchive   = core.OpArchive
	OpExtract   = core.OpExtract
	OpSearch    = core.OpSearch
	OpCalculate = core.OpCalculate
)

type OperationStatus = core.OperationStatus

const (
	StatusPending   = core.StatusPending
	StatusRunning   = core.StatusRunning
	StatusPaused    = core.StatusPaused
	StatusCompleted = core.StatusCompleted
	StatusCancelled = core.StatusCancelled
	StatusError     = core.StatusError
)

type OperationProgress = core.OperationProgress
