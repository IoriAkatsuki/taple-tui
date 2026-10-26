package motion

import (
	"math"
	"testing"
)

func TestMechanicalBackEasing(t *testing.T) {
	if math.Abs(InBack(0)) > 1e-14 || math.Abs(InBack(1)-1) > 1e-14 || math.Abs(OutBack(0)) > 1e-14 || math.Abs(OutBack(1)-1) > 1e-14 {
		t.Fatal("endpoints")
	}
	if InBack(.3) >= 0 || OutBack(.7) <= 1 {
		t.Fatal("missing mechanical overshoot")
	}
}
