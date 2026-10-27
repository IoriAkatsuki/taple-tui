//go:build (linux || windows) && cgo

// Package vkcompute owns a headless compute queue and persistent host buffers.
package vkcompute

/*
#cgo linux LDFLAGS: -ldl
#include "compute.h"
*/
import "C"

import (
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"unsafe"
)

type Device struct {
	mu   sync.Mutex
	ptr  *C.tf_gpu
	name string
}

func Open(code []byte) (*Device, error) {
	if len(code) < 20 || len(code)%4 != 0 || binary.LittleEndian.Uint32(code) != 0x07230203 {
		return nil, errors.New("invalid SPIR-V")
	}
	var message [256]C.char
	// pCode requires uint32 alignment; an embedded byte slice need not provide it.
	words := make([]uint32, len(code)/4)
	for i := range words {
		words[i] = binary.LittleEndian.Uint32(code[i*4:])
	}
	p := C.tf_gpu_open((*C.uint32_t)(unsafe.Pointer(&words[0])), C.size_t(len(code)), &message[0], 256)
	if p == nil {
		return nil, errors.New(C.GoString(&message[0]))
	}
	return &Device{ptr: p, name: C.GoString(C.tf_gpu_name(p))}, nil
}

func (d *Device) Name() string { return d.name }
func (d *Device) Compute(params []float32, out []float32) error {
	if len(params) != 128 || len(out) == 0 || len(out)%4 != 0 || len(out)/4 > 1<<20 {
		return errors.New("invalid compute dimensions")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.ptr == nil {
		return errors.New("Vulkan device closed")
	}
	result := C.tf_gpu_run(d.ptr, (*C.float)(unsafe.Pointer(&params[0])), (*C.float)(unsafe.Pointer(&out[0])), C.uint32_t(len(out)/4))
	if result != 0 {
		return fmt.Errorf("Vulkan dispatch failed (%d)", int(result))
	}
	return nil
}
func (d *Device) Close() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.ptr != nil {
		C.tf_gpu_close(d.ptr)
		d.ptr = nil
	}
}
