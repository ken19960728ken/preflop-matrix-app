package paritydiff

import (
	"reflect"
	"strings"
	"testing"
)

func TestNormalizeSummary(t *testing.T) {
	in := strings.Join([]string{
		"理論範圍來源：參數 --ranges → x",
		"手牌：掃描 3 個檔案",
		"  無法解析範例：bad.txt: unsupported game or bad header: 'x'",
		"理論範圍：5 個檔案",
		"  無法讀取：BTN vs BB 3bet call.txt: [Errno 21] Is a directory: 'p'",
		"  無法讀取：no reason.txt",
		"  未填（空白檔）：a.txt",
		"已輸出：/tmp/x.html、/tmp/x.json",
		"",
	}, "\r\n")
	want := []string{
		"理論範圍來源：參數 --ranges → x",
		"手牌：掃描 3 個檔案",
		"  無法解析範例：bad.txt",
		"理論範圍：5 個檔案",
		"  無法讀取：BTN vs BB 3bet call.txt",
		"  無法讀取：no reason.txt",
		"  未填（空白檔）：a.txt",
	}
	if got := NormalizeSummary(in); !reflect.DeepEqual(got, want) {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func TestCompareSummary(t *testing.T) {
	base := "手牌：掃描 1 個檔案\n  無法讀取：a.txt: python reason\n  未填（空白檔）：b.txt\n已輸出：/x\n"
	tests := []struct {
		name, a, b string
		want       int
	}{
		{"reason and output path ignored", base, "手牌：掃描 1 個檔案\n  無法讀取：a.txt: 另一個原因\n  未填（空白檔）：b.txt\n已輸出：/y\n", 0},
		{"file name differs", base, strings.Replace(base, "a.txt", "c.txt", 1), 1},
		{"order matters", base, "手牌：掃描 1 個檔案\n  未填（空白檔）：b.txt\n  無法讀取：a.txt\n", 2},
		{"count differs", base, strings.Replace(base, "1 個", "2 個", 1), 1},
		{"extra line in b", base, base + "  重複：z.txt\n", 1},
		{"missing line in b", base, "手牌：掃描 1 個檔案\n", 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := CompareSummary(tc.a, tc.b); len(got) != tc.want {
				t.Errorf("want %d diffs, got %d: %q", tc.want, len(got), got)
			}
		})
	}
}
