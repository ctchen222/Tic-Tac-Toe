package response

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Response struct {
	Success bool `json:"success"`
	Code    int  `json:"code"`
	Extras  any  `json:"extras"`
}

func NewResponse(success bool, code int, extras any) Response {
	return Response{
		Success: success,
		Code:    code,
		Extras:  extras,
	}
}

// SuccessResponseContent returns a JSON response with a success message and content
func SuccessResponseContent(c *gin.Context, content string) {
	c.JSON(
		http.StatusOK,
		NewResponse(
			true,
			http.StatusOK,
			map[string]interface{}{
				"content": content,
			},
		))
}

// SuccessResponseList returns a JSON response with a success message and a list of items
func SuccessResponseList[T []any | map[string]any](c *gin.Context, list T) {
	c.JSON(
		http.StatusOK,
		NewResponse(
			true,
			http.StatusOK,
			map[string]any{
				"list": list,
			},
		))
}

// SuccessResponse returns a JSON response with a success message with no type limitation
func SuccessResponse(c *gin.Context, extras any) {
	c.JSON(
		http.StatusOK,
		NewResponse(
			true,
			http.StatusOK,
			extras,
		))
}

func ErrorResponse(c *gin.Context, code int, message string) {
	c.JSON(
		code,
		NewResponse(
			false,
			code,
			map[string]interface{}{
				"message": message,
			},
		))
}
