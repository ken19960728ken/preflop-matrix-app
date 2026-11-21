package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"strings"
)

const (
	ZhHant = "zh-Hant"
	ZhHans = "zh-Hans"
	En     = "en"
)

// Languages 是支援的語言，順序即語言選擇器的順序。
var Languages = []string{ZhHant, ZhHans, En}

//go:embed locales/*/*.json
var localesFS embed.FS

// catalogs[lang][key]：同一語言下所有命名空間（cli、engine、report、ui…）合併；key 自帶命名空間前綴。
var catalogs = mustLoad(localesFS)

var placeholder = regexp.MustCompile(`\{([A-Za-z0-9_]+)\}`)

func mustLoad(fsys fs.FS) map[string]map[string]string {
	out := make(map[string]map[string]string, len(Languages))
	for _, lang := range Languages {
		merged := map[string]string{}
		files, err := fs.Glob(fsys, path.Join("locales", lang, "*.json"))
		if err != nil {
			panic(err)
		}
		for _, file := range files {
			raw, err := fs.ReadFile(fsys, file)
			if err != nil {
				panic(err)
			}
			var ns map[string]string
			if err := json.Unmarshal(raw, &ns); err != nil {
				panic(fmt.Sprintf("i18n: %s: %v", file, err))
			}
			for k, v := range ns {
				merged[k] = v
			}
		}
		out[lang] = merged
	}
	return out
}

// Locales 回傳內嵌的 locales 根目錄（<lang>/<namespace>.json），供 report.LoadDict 使用。
func Locales() fs.FS {
	sub, err := fs.Sub(localesFS, "locales")
	if err != nil {
		panic(err)
	}
	return sub
}

// IsSupported 回報 lang 是否剛好是三個語言代碼之一（不做 Normalize）。
func IsSupported(lang string) bool {
	_, ok := catalogs[lang]
	return ok
}

// T 取得 key 在 lang 的字串並代入 {name} 佔位符；缺 key 退回 en，再退回 key 本身。
// args 裡沒有的佔位符保持原樣；代入值不會被再次展開。
func T(lang, key string, args map[string]string) string {
	text, ok := catalogs[Normalize(lang)][key]
	if !ok {
		text, ok = catalogs[En][key]
	}
	if !ok {
		return key
	}
	if len(args) == 0 {
		return text
	}
	return placeholder.ReplaceAllStringFunc(text, func(m string) string {
		if v, found := args[m[1:len(m)-1]]; found {
			return v
		}
		return m
	})
}

// Normalize 把任意語言標籤（zh-TW、zh_TW.UTF-8、en-US…）對應到三個支援語言之一。
// 繁體：腳本 Hant，或地區 TW/HK/MO；其餘 zh（含 zh、zh-CN、zh-SG、zh-Hans）為簡體；其他一律 en。
func Normalize(tag string) string {
	tag = strings.ToLower(tag)
	if i := strings.IndexAny(tag, ".@"); i >= 0 {
		tag = tag[:i]
	}
	parts := strings.FieldsFunc(tag, func(r rune) bool { return r == '-' || r == '_' })
	if len(parts) == 0 || parts[0] != "zh" {
		return En
	}
	hant := false
	for _, p := range parts[1:] {
		switch p {
		case "hans":
			return ZhHans
		case "hant", "tw", "hk", "mo":
			hant = true
		}
	}
	if hant {
		return ZhHant
	}
	return ZhHans
}
