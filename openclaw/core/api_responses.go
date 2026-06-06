package core

type OperationStatusResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Error   string `json:"error"`
	Mode    string `json:"mode"`
}
