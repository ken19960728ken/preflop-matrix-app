package engine

import (
	"math"
	"reflect"
	"testing"
)

func sameFloat(a, b float64) bool {
	return math.Float64bits(a) == math.Float64bits(b) || (math.IsNaN(a) && math.IsNaN(b))
}

// Vectors in pyvectors_test.go come from CPython 3.12.12.

func TestPySum(t *testing.T) {
	for i, v := range pySumVectors {
		if got := pySum(v.in); !sameFloat(got, v.want) {
			t.Errorf("vector %d: pySum = %v (%#x), want %v (%#x)", i, got, math.Float64bits(got), v.want, math.Float64bits(v.want))
		}
	}
	if got := pySum([]float64{1e100, 1.0, -1e100}); got != 1.0 {
		t.Errorf("Neumaier compensation lost: %v", got)
	}
}

func TestParsePyFloat(t *testing.T) {
	for _, v := range parseFloatVectors {
		got, ok := parsePyFloat(v.in)
		if ok != v.ok || (ok && !sameFloat(got, v.want)) {
			t.Errorf("parsePyFloat(%q) = %v, %v; want %v, %v", v.in, got, ok, v.want, v.ok)
		}
	}
}

func TestPySpaceMatchesPython(t *testing.T) {
	want := map[rune]bool{}
	for _, r := range pySpaceRunes {
		want[r] = true
	}
	for r := rune(0); r <= 0x10FFFF; r++ {
		if isPySpace(r) != want[r] {
			t.Fatalf("isPySpace(%U) = %v, want %v", r, isPySpace(r), want[r])
		}
	}
}

func TestStripAndSplit(t *testing.T) {
	for _, v := range stripSplitVectors {
		if got := pyStrip(v.in); got != v.strip {
			t.Errorf("pyStrip(%q) = %q, want %q", v.in, got, v.strip)
		}
		if got := pyRStrip(v.in); got != v.rstrip {
			t.Errorf("pyRStrip(%q) = %q, want %q", v.in, got, v.rstrip)
		}
		if got := pySplit(v.in); !reflect.DeepEqual(got, append([]string(nil), v.split...)) && !(len(got) == 0 && len(v.split) == 0) {
			t.Errorf("pySplit(%q) = %q, want %q", v.in, got, v.split)
		}
	}
}

func TestPyRepr(t *testing.T) {
	for _, v := range reprVectors {
		if got := pyRepr(v.in); got != v.want {
			t.Errorf("pyRepr(%q) = %s, want %s", v.in, got, v.want)
		}
	}
}

func TestDecimalDigit(t *testing.T) {
	for _, zero := range decimalZeros {
		for d := 0; d < 10; d++ {
			if got, ok := decimalDigit(zero + rune(d)); !ok || got != d {
				t.Fatalf("decimalDigit(%U) = %d, %v; want %d", zero+rune(d), got, ok, d)
			}
		}
	}
	if _, ok := decimalDigit('a'); ok {
		t.Error("a is not a digit")
	}
	if _, ok := decimalDigit('²'); ok {
		t.Error("superscript two is not a decimal digit")
	}
}

func TestPyMin(t *testing.T) {
	if pyMin(1.0, 2.0) != 1.0 || pyMin(2.0, 1.0) != 1.0 || pyMin(1.0, 1.0) != 1.0 {
		t.Error("pyMin")
	}
	if math.Signbit(pyMin(0.0, negZero)) {
		t.Error("min(0.0, -0.0) is 0.0: -0.0 is not smaller")
	}
}
