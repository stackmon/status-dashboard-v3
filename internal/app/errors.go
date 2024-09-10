package app

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

var ErrIncidentDSNotExist = errors.New("incident does not exist")

var ErrComponentDSNotExist = errors.New("component does not exist")
var ErrComponentInvalidFormat = errors.New("component invalid format")

func Return404(c *gin.Context) {
	c.JSON(http.StatusNotFound, ReturnError(ErrPageNotFound))
}

func raiseInternalErr(c *gin.Context, err error) {
	intErr := fmt.Errorf("%w: %w", ErrInternalError, err)
	_ = c.AbortWithError(http.StatusInternalServerError, ReturnError(intErr))
}

func raiseBadRequestErr(c *gin.Context, err error) {
	_ = c.AbortWithError(http.StatusBadRequest, ReturnError(err))
}

func raiseStatusNotFoundErr(c *gin.Context, err error) {
	_ = c.AbortWithError(http.StatusNotFound, ReturnError(err))
}
