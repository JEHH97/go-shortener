package domain

import "errors"

var ErrInvalidCodeSize = errors.New("code size must be between 1 and 32")
