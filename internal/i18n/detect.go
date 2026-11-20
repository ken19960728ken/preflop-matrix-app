package i18n

import "strings"

// localeName 去掉 ".encoding" 與 "@modifier"；"C"、"POSIX"（含 "C.UTF-8"）視為未設定，回傳空字串。
func localeName(v string) string {
	v = strings.TrimSpace(v)
	if i := strings.IndexAny(v, ".@"); i >= 0 {
		v = v[:i]
	}
	if v == "C" || v == "POSIX" {
		return ""
	}
	return v
}

// detectFromEnv 依序看 LANGUAGE（冒號分隔，取第一個有效項）、LC_ALL、LC_MESSAGES、LANG，取第一個有效值，最後退回 en。
func detectFromEnv(getenv func(string) string) string {
	for _, item := range strings.Split(getenv("LANGUAGE"), ":") {
		if name := localeName(item); name != "" {
			return Normalize(name)
		}
	}
	for _, key := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if name := localeName(getenv(key)); name != "" {
			return Normalize(name)
		}
	}
	return En
}
