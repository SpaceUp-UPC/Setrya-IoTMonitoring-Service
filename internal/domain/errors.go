package domain

import "errors"

var (
	ErrInvalid           = errors.New("invalid data")
	ErrDeviceNotFound    = errors.New("device not found")
	ErrDeviceExists      = errors.New("device already exists")
	ErrDeviceInactive    = errors.New("device is not active")
	ErrReadingNotFound   = errors.New("reading not found")
	ErrAlertNotFound     = errors.New("alert not found")
	ErrInvalidTransition = errors.New("invalid alert state transition")
	ErrNotValidatable    = errors.New("alert has no security event to validate")
	ErrBrokerUnavailable = errors.New("mqtt broker unavailable")
)
