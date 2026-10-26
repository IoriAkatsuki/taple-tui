//go:build (!linux && !windows && !darwin) || !cgo

package vkcompute

import "errors"

type Device struct{}

func Open([]byte) (*Device, error) {
	return nil, errors.New("Vulkan requires a Linux, Windows or macOS CGO build")
}
func (*Device) Name() string                       { return "" }
func (*Device) Compute([]float32, []float32) error { return errors.New("Vulkan unavailable") }
func (*Device) Close()                             {}
