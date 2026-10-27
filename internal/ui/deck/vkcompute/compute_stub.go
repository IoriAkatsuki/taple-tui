//go:build (!linux && !windows) || !cgo

package vkcompute

import "errors"

type Device struct{}

func Open([]byte) (*Device, error) {
	return nil, errors.New("Vulkan requires a Linux or Windows CGO build; macOS uses Metal")
}
func (*Device) Name() string                       { return "" }
func (*Device) Compute([]float32, []float32) error { return errors.New("Vulkan unavailable") }
func (*Device) Close()                             {}
