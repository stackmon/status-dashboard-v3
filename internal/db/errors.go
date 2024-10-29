package db

import "errors"

var ErrDBComponentDSNotExist = errors.New("component does not exist")
var ErrDBComponentExists = errors.New("component exists")
var ErrDBIncidentDSNotExist = errors.New("incident does not exist")
