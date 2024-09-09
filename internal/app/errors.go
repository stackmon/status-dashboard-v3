package app

import (
	"errors"
)

func ReturnError(err error) error {
	return &ErrorMsg{ErrMsg: err.Error()}
}

type ErrorMsg struct {
	ErrMsg string `json:"errMsg"`
}

func (e *ErrorMsg) Error() string {
	return e.ErrMsg
}

var ErrPageNotFound = errors.New("page not found")

var ErrComponentIsNotPresent = errors.New("component is not present")
