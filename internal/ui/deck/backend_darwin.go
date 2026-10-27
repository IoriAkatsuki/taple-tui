//go:build darwin

package deck

import (
	_ "embed"

	"github.com/go-musicfox/go-musicfox/internal/ui/deck/metalcompute"
)

//go:embed walkman.metal
var walkmanMSL string

func GPUBackend() string { return "Metal" }

func openNativeGPU() (computeDevice, error) { return metalcompute.Open(walkmanMSL) }
