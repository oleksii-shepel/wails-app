package contracts

// TrashTransferOperation describes a transfer out of the OS Recycle Bin.
// Copy leaves the trash entry intact; restore removes it from the Bin.
type TrashTransferOperation string

const (
	TrashTransferCopy    TrashTransferOperation = "copy"
	TrashTransferRestore TrashTransferOperation = "restore"
)

type TrashTransferRequest struct {
	IDs            []string               `json:"ids"`
	DestinationDir string                 `json:"destinationDir"`
	Operation      TrashTransferOperation `json:"operation"`
}

type TrashTransferItemError struct {
	ID    string `json:"id"`
	Error string `json:"error"`
}

type TrashTransferResult struct {
	OperationID  string                   `json:"operationId"`
	Total        int                      `json:"total"`
	Succeeded    int                      `json:"succeeded"`
	Failed       int                      `json:"failed"`
	Destinations []string                 `json:"destinations,omitempty"`
	Errors       []TrashTransferItemError `json:"errors,omitempty"`
}
