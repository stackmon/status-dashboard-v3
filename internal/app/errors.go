package app

import (
	"errors"
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

var ErrComponentIsNotPresent = errors.New("component is not present")
