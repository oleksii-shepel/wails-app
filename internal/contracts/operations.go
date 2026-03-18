package contracts

// OperationType categorizes long-running operations.
type OperationType string

const (
	OpCopy      OperationType = "COPY"
	OpMove      OperationType = "MOVE"
	OpDelete    OperationType = "DELETE"
	OpArchive   OperationType = "ARCHIVE"
	OpExtract   OperationType = "EXTRACT"
	OpSearch    OperationType = "SEARCH"
	OpCalculate OperationType = "CALCULATE"
)

// OperationStatus represents the current state.
type OperationStatus string

const (
	StatusPending   OperationStatus = "PENDING"
	StatusRunning   OperationStatus = "RUNNING"
	StatusPaused    OperationStatus = "PAUSED"
	StatusCompleted OperationStatus = "COMPLETED"
	StatusCancelled OperationStatus = "CANCELLED"
	StatusError     OperationStatus = "ERROR"
)

// OperationProgress holds all progress state for an operation.
type OperationProgress struct {
	ID              string          `json:"id"`
	Type            OperationType   `json:"type"`
	Status          OperationStatus `json:"status"`
	Title           string          `json:"title"`
	SourcePath      string          `json:"sourcePath"`
	DestinationPath string          `json:"destinationPath,omitempty"`

	TotalItems      int64  `json:"totalItems"`
	ProcessedItems  int64  `json:"processedItems"`
	TotalBytes      int64  `json:"totalBytes"`
	ProcessedBytes  int64  `json:"processedBytes"`
	CurrentFile     string `json:"currentFile"`
	CurrentFileSize int64  `json:"currentFileSize"`

	PercentComplete     float64 `json:"percentComplete"`
	SpeedBytesPerSec    float64 `json:"speedBytesPerSec"`
	AvgSpeedBytesPerSec float64 `json:"avgSpeedBytesPerSec"`
	ETASeconds          int64   `json:"etaSeconds"`

	StartTime      int64  `json:"startTime"`
	ElapsedSeconds int64  `json:"elapsedSeconds"`
	EndTime        *int64 `json:"endTime,omitempty"`

	Error *string `json:"error,omitempty"`
}
