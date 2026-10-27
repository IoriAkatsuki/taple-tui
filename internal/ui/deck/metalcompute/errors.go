package metalcompute

import "errors"

// ErrUnavailable means no native Metal build/device, not a broken shader.
var ErrUnavailable = errors.New("Metal unavailable")
