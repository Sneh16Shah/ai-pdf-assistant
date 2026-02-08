// Code generated manually for MVP - matches proto/common.proto
package proto

// Status represents the status of an operation
type Status int32

const (
	Status_STATUS_UNKNOWN    Status = 0
	Status_STATUS_SUCCESS     Status = 1
	Status_STATUS_ERROR       Status = 2
	Status_STATUS_NOT_FOUND   Status = 3
	Status_STATUS_PROCESSING  Status = 4
)

// Error represents error details
type Error struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

// Pagination represents pagination information
type Pagination struct {
	Page     int32 `json:"page"`
	PageSize int32 `json:"page_size"`
	Total    int32 `json:"total"`
}

