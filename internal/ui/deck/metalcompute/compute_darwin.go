//go:build darwin && cgo

// Package metalcompute uses macOS Metal directly, without Vulkan or a window.
package metalcompute

/*
#cgo LDFLAGS: -framework Foundation -framework Metal
#include <stdlib.h>
#include "compute.h"
*/
import "C"

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"unsafe"
)

type Device struct {
	mu   sync.Mutex
	ptr  *C.tf_metal
	name string
}

func Open(source string) (*Device, error) {
	if source == "" || strings.ContainsRune(source, '\x00') {
		return nil, errors.New("invalid Metal shader")
	}
	code := C.CString(source)
	defer C.free(unsafe.Pointer(code))
	var message [512]C.char
	p := C.tf_metal_open(code, &message[0], C.size_t(len(message)))
	if p == nil {
		detail := C.GoString(&message[0])
		if detail == "Metal device unavailable" {
			return nil, fmt.Errorf("%w: no native device", ErrUnavailable)
		}
		return nil, errors.New(detail)
	}
	return &Device{ptr: p, name: C.GoString(C.tf_metal_name(p))}, nil
}

func (d *Device) Name() string { return d.name }

func (d *Device) Compute(params []float32, out []float32) error {
	if len(params) != 128 || len(out) == 0 || len(out)%4 != 0 || len(out)/4 > 1<<20 {
		return errors.New("invalid compute dimensions")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.ptr == nil {
		return errors.New("Metal device closed")
	}
	var message [512]C.char
	if C.tf_metal_run(d.ptr, (*C.float)(unsafe.Pointer(&params[0])), (*C.float)(unsafe.Pointer(&out[0])), C.uint32_t(len(out)/4), &message[0], C.size_t(len(message))) != 0 {
		return errors.New(C.GoString(&message[0]))
	}
	return nil
}

func (d *Device) Close() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.ptr != nil {
		C.tf_metal_close(d.ptr)
		d.ptr = nil
	}
}
