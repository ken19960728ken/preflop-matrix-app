package numfmt

import (
	"math"
	"testing"
)

func TestToFixed1MatchesJavaScript(t *testing.T) {
	cases := map[float64]string{
		0: "0.0", 0.04: "0.0", 0.05: "0.1", 0.25: "0.3", 0.35: "0.3", 0.75: "0.8", 1.25: "1.3", 2.5: "2.5",
		-0.25: "-0.3", -0.04: "-0.0", -12.349: "-12.3", 12.35: "12.3", 99.95: "100.0", 3.0000001: "3.0", 1234.56: "1234.6",
		1e15: "1000000000000000.0",
	}
	for in, want := range cases {
		if got := ToFixed1(in); got != want {
			t.Errorf("ToFixed1(%v) = %q, want %q", in, got, want)
		}
	}
}

func TestSignedAndPercent(t *testing.T) {
	signed := map[float64]string{0: "0.0", -0.04: "0.0", 0.04: "0.0", 0.05: "+0.1", -0.05: "-0.1", 12.34: "+12.3", -3.26: "-3.3"}
	for in, want := range signed {
		if got := Signed(in); got != want {
			t.Errorf("Signed(%v) = %q, want %q", in, got, want)
		}
	}
	percent := map[float64]string{0: "0.0%", 0.5: "50.0%", 0.12345: "12.3%", 1: "100.0%", 0.0025: "0.3%"}
	for in, want := range percent {
		if got := Percent(in); got != want {
			t.Errorf("Percent(%v) = %q, want %q", in, got, want)
		}
	}
}

func TestNonFiniteValues(t *testing.T) {
	for _, v := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if ToFixed1(v) != NotANumber || Signed(v) != NotANumber || Percent(v) != NotANumber {
			t.Errorf("%v: %q %q %q", v, ToFixed1(v), Signed(v), Percent(v))
		}
	}
}
