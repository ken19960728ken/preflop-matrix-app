package paritydiff

import (
	"fmt"
	"strings"
)

// 這些摘要行的結尾是「檔名: 原因」；原因（Python 例外原文、Go 三語化訊息）不比，只比到檔名。
var reasonPrefixes = []string{"  無法讀取：", "  無法解析範例："}

// 內容與輸出位置有關、不屬於分類的行。
const outputLinePrefix = "已輸出："

// NormalizeSummary 把 CLI stdout 正規化成可逐行比對的行：
// 統一換行、丟掉「已輸出：…」行，並把「無法讀取／無法解析範例」行截掉「檔名: 」之後的原因。
func NormalizeSummary(stdout string) []string {
	stdout = strings.ReplaceAll(strings.ReplaceAll(stdout, "\r\n", "\n"), "\r", "\n")
	var out []string
	for _, line := range strings.Split(strings.TrimSuffix(stdout, "\n"), "\n") {
		if strings.HasPrefix(line, outputLinePrefix) {
			continue
		}
		for _, p := range reasonPrefixes {
			if rest, ok := strings.CutPrefix(line, p); ok {
				name, _, _ := strings.Cut(rest, ": ")
				line = p + name
				break
			}
		}
		out = append(out, line)
	}
	return out
}

// CompareSummary 正規化兩份 stdout 後依序逐行比對，回傳每一處差異的描述（無差異回 nil）。
func CompareSummary(a, b string) []string {
	la, lb := NormalizeSummary(a), NormalizeSummary(b)
	var diffs []string
	for i := 0; i < len(la) || i < len(lb); i++ {
		switch {
		case i >= len(lb):
			diffs = append(diffs, fmt.Sprintf("line %d: only in a: %s", i+1, la[i]))
		case i >= len(la):
			diffs = append(diffs, fmt.Sprintf("line %d: only in b: %s", i+1, lb[i]))
		case la[i] != lb[i]:
			diffs = append(diffs, fmt.Sprintf("line %d:\n  a: %s\n  b: %s", i+1, la[i], lb[i]))
		}
	}
	return diffs
}
