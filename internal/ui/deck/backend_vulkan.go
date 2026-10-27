//go:build !darwin

package deck

import (
	_ "embed"

	"github.com/go-musicfox/go-musicfox/internal/ui/deck/vkcompute"
)

//go:embed walkman.comp.spv
var walkmanSPIRV []byte

func GPUBackend() string { return "Vulkan" }

func openNativeGPU() (computeDevice, error) { return vkcompute.Open(walkmanSPIRV) }
