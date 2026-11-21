// Package numfmt: 偏離摘要的數字格式（JavaScript toFixed 語意，與報告頁一致）。葉節點套件，不 import 專案內其他套件。
package numfmt

import (
	"math"
	"strconv"
	"strings"
)

// NotANumber 是 NaN 與 ±Inf 的顯示。
const NotANumber = "–"

// ToFixed1 等同 JavaScript 的 Number.prototype.toFixed(1)：以精確十進位值四捨五入，剛好在中間時遠離 0
// （Go 的 %.1f 在中間值時取偶數，例如 0.25 會得到 0.2，報告頁是 0.3）。NaN、±Inf 回傳 NotANumber。
func ToFixed1(v float64) string {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return NotANumber
	}
	exact := strconv.FormatFloat(math.Abs(v), 'f', 25, 64)
	whole, frac, _ := strings.Cut(exact, ".")
	tenths, err := strconv.ParseUint(whole+frac[:1], 10, 64)
	if err != nil {
		return strconv.FormatFloat(v, 'f', 1, 64)
	}
	if frac[1] >= '5' {
		tenths++
	}
	s := strconv.FormatUint(tenths/10, 10) + "." + strconv.FormatUint(tenths%10, 10)
	if v < 0 {
		return "-" + s
	}
	return s
}

// Signed 同報告頁的 signed(v, 1)：帶正負號的一位小數；四捨五入成 0 時一律是 "0.0"。
func Signed(v float64) string {
	s := ToFixed1(v)
	switch {
	case s == NotANumber:
		return s
	case s == "0.0" || s == "-0.0":
		return "0.0"
	case strings.HasPrefix(s, "-"):
		return s
	}
	return "+" + s
}

// Percent 同報告頁的 (v * 100).toFixed(1) + "%"。
func Percent(v float64) string {
	s := ToFixed1(float64(v * 100))
	if s == NotANumber {
		return s
	}
	return s + "%"
}
