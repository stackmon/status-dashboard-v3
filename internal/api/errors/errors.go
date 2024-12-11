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

var ErrIncidentDSNotExist = errors.New("incident does not exist")
var ErrIncidentEndDateShouldBeEmpty = errors.New("incident end_date should be empty")
var ErrIncidentUpdatesShouldBeEmpty = errors.New("incident updates should be empty")

var ErrIncidentCreationMaintenanceExists = errors.New("incident creation failed, component in maintenance incident")
var ErrIncidentCreationLowImpact = errors.New(
	"incident creation failed, exists the incident with higher impact for component",
)

var ErrComponentDSNotExist = errors.New("component does not exist")

func NewErrComponentDSNotExist(componentID int) error {
	return fmt.Errorf("%w, component_id: %d", ErrComponentDSNotExist, componentID)
}

var ErrComponentExist = errors.New("component already exists")
var ErrComponentInvalidFormat = errors.New("component invalid format")
var ErrComponentAttrInvalidFormat = errors.New("component attribute has invalid format")
var ErrComponentRegionAttrMissing = errors.New("component attribute region is missing or invalid")
var ErrComponentTypeAttrMissing = errors.New("component attribute type is missing or invalid")
var ErrComponentCategoryAttrMissing = errors.New("component attribute category is missing or invalid")

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
