//go:build !darwin || !cgo

package metalcompute

import "fmt"

type Device struct{}

func Open(string) (*Device, error) {
	return nil, fmt.Errorf("%w: requires a macOS CGO build", ErrUnavailable)
}
func (*Device) Name() string                       { return "" }
func (*Device) Compute([]float32, []float32) error { return ErrUnavailable }
func (*Device) Close()                             {}
