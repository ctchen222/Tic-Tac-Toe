package response

type Error struct {
	Success bool   `json:"success"`
	Code    int    `json:"code"`
	Extras  string `json:"extras"`
}

func (e Error) Error() string {
	return e.Extras
}

type ErrorOptions struct {
	Code    int
	Message string
}

func NewError(success bool, code int, message string) Error {
	return Error{
		Success: success,
		Code:    code,
		Extras:  message,
	}
}
