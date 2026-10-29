//go:build darwin && cgo

package metalcompute

import (
	"errors"
	"os"
	"testing"
)

func TestMetalRoundTripAndClose(t *testing.T) {
	d, err := Open(`#include <metal_stdlib>
using namespace metal;
kernel void main0(device const float *p [[buffer(0)]], device float4 *out [[buffer(1)]], uint i [[thread_position_in_grid]]) {
  if (i < uint(p[0])) out[i] = float4(float(i), p[1], p[2], 1.0);
}`)
	if err != nil {
		if !errors.Is(err, ErrUnavailable) || os.Getenv("TAPLE_REQUIRE_GPU") == "1" {
			t.Fatal(err)
		}
		t.Skip(err)
	}
	defer d.Close()
	var p [128]float32
	p[1], p[2] = 2, 3
	for _, n := range []int{1, 65, 257, 32} {
		p[0] = float32(n)
		out := make([]float32, n*4)
		if err := d.Compute(p[:], out); err != nil {
			t.Fatal(err)
		}
		for i := range n {
			want := [4]float32{float32(i), 2, 3, 1}
			for j, value := range want {
				if out[i*4+j] != value {
					t.Fatalf("n=%d pixel=%d channel=%d: got %v want %v", n, i, j, out[i*4+j], value)
				}
			}
		}
	}
	if err := d.Compute(p[:127], make([]float32, 4)); err == nil {
		t.Fatal("invalid dimensions accepted")
	}
	d.Close()
	d.Close()
	if err := d.Compute(p[:], make([]float32, 4)); err == nil {
		t.Fatal("closed device accepted dispatch")
	}
}

func TestMetalRejectsInvalidSource(t *testing.T) {
	for _, source := range []string{"", "invalid\x00source"} {
		if _, err := Open(source); err == nil {
			t.Fatal("invalid source accepted")
		}
	}
}
