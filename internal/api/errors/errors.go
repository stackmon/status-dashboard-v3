package errors

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

func ReturnError(err error) error {
	return &MsgError{Msg: err.Error()}
}

type MsgError struct {
	Msg string `json:"errMsg"`
}

func (e *MsgError) Error() string {
	return e.Msg
}

var ErrPageNotFound = errors.New("page not found")
var ErrInternalError = errors.New("internal server error")

func Return404(c *gin.Context) {
	c.JSON(http.StatusNotFound, ReturnError(ErrPageNotFound))
}

func RaiseInternalErr(c *gin.Context, err error) {
	intErr := fmt.Errorf("%w: %w", ErrInternalError, err)
	_ = c.AbortWithError(http.StatusInternalServerError, ReturnError(intErr))
}

func RaiseBadRequestErr(c *gin.Context, err error) {
	_ = c.AbortWithError(http.StatusBadRequest, ReturnError(err))
}

func RaiseStatusNotFoundErr(c *gin.Context, err error) {
	_ = c.AbortWithError(http.StatusNotFound, ReturnError(err))
}
