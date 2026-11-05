package domain

import "errors"

var (
	ErrInvalid           = errors.New("invalid data")
	ErrDeviceNotFound    = errors.New("device not found")
	ErrDeviceInactive    = errors.New("device is not active")
	ErrAlertNotFound     = errors.New("alert not found")
	ErrInvalidTransition = errors.New("invalid alert state transition")
)
