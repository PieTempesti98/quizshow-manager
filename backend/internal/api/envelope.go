package api

// DataResponse wraps a successful response payload in the standard envelope.
type DataResponse struct {
	Data any `json:"data"`
}

// ErrorDetail is the structured error object inside ErrorResponse.
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ErrorResponse wraps an error in the standard envelope.
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}
