package i18n

import (
	"encoding/json"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"testing/fstest"
	"unicode"
)

func TestNormalize(t *testing.T) {
	cases := map[string]string{
		"zh-TW":        ZhHant,
		"zh-HK":        ZhHant,
		"zh-MO":        ZhHant,
		"zh-Hant":      ZhHant,
		"zh-Hant-TW":   ZhHant,
		"zh-Hant-CN":   ZhHant,
		"zh_TW.UTF-8":  ZhHant,
		"ZH-tw":        ZhHant,
		"zh-CN":        ZhHans,
		"zh-SG":        ZhHans,
		"zh":           ZhHans,
		"zh-Hans":      ZhHans,
		"zh-Hans-HK":   ZhHans,
		"zh_CN.UTF-8":  ZhHans,
		"en-US":        En,
		"en":           En,
		"ja":           En,
		"zha":          En,
		"":             En,
		"C":            En,
		"de_DE@euro":   En,
		"zh_TW@modif":  ZhHant,
		"-":            En,
		"fr-CA":        En,
		"zh-Hant-HK.x": ZhHant,
	}
	for in, want := range cases {
		if got := Normalize(in); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTFallbackAndSubstitution(t *testing.T) {
	prev := catalogs
	t.Cleanup(func() { catalogs = prev })
	catalogs = map[string]map[string]string{
		ZhHant: {"a": "甲 {x} {y}"},
		ZhHans: {},
		En:     {"a": "A {x}", "only_en": "E", "braces": "{x}"},
	}
	cases := []struct {
		lang, key string
		args      map[string]string
		want      string
	}{
		{ZhHant, "a", map[string]string{"x": "1", "y": "2"}, "甲 1 2"},
		{"zh-TW", "a", map[string]string{"x": "1"}, "甲 1 {y}"},
		{ZhHans, "a", map[string]string{"x": "1"}, "A 1"},
		{ZhHant, "only_en", nil, "E"},
		{"xx", "only_en", nil, "E"},
		{En, "missing", nil, "missing"},
		{En, "braces", map[string]string{"x": "{x}{y}", "y": "!"}, "{x}{y}"},
	}
	for _, c := range cases {
		if got := T(c.lang, c.key, c.args); got != c.want {
			t.Errorf("T(%q,%q,%v) = %q, want %q", c.lang, c.key, c.args, got, c.want)
		}
	}
}

func TestIsSupported(t *testing.T) {
	for _, l := range Languages {
		if !IsSupported(l) {
			t.Errorf("%s should be supported", l)
		}
	}
	if IsSupported("zh-TW") || IsSupported("") {
		t.Error("only exact language codes are supported")
	}
}

func TestDetectFromEnv(t *testing.T) {
	cases := []struct {
		env  map[string]string
		want string
	}{
		{map[string]string{}, En},
		{map[string]string{"LANG": "zh_TW.UTF-8"}, ZhHant},
		{map[string]string{"LANG": "en_US.UTF-8", "LC_ALL": "zh_CN.UTF-8"}, ZhHans},
		{map[string]string{"LC_ALL": "C", "LC_MESSAGES": "zh_HK", "LANG": "en_US"}, ZhHant},
		{map[string]string{"LC_ALL": "POSIX", "LANG": "zh_CN"}, ZhHans},
		{map[string]string{"LC_ALL": "  ", "LANG": "ja_JP"}, En},
		{map[string]string{"LC_ALL": "C", "LANG": "zh_TW"}, ZhHant},
		{map[string]string{"LC_ALL": "C.UTF-8", "LANG": "zh_CN.UTF-8"}, ZhHans},
		{map[string]string{"LC_ALL": "POSIX", "LC_MESSAGES": "C.UTF-8", "LANG": "zh_TW"}, ZhHant},
		{map[string]string{"LANG": "C.UTF-8"}, En},
		{map[string]string{"LANG": "POSIX"}, En},
		{map[string]string{"LANGUAGE": "zh_TW:en", "LANG": "en_US.UTF-8"}, ZhHant},
		{map[string]string{"LANGUAGE": ":C:zh_CN:en", "LC_ALL": "zh_TW"}, ZhHans},
		{map[string]string{"LANGUAGE": "C", "LC_ALL": "zh_TW"}, ZhHant},
		{map[string]string{"LANGUAGE": "en", "LC_ALL": "zh_TW"}, En},
		{map[string]string{"LANGUAGE": "zh_TW@x.y"}, ZhHant},
	}
	for _, c := range cases {
		if got := detectFromEnv(func(k string) string { return c.env[k] }); got != c.want {
			t.Errorf("detectFromEnv(%v) = %q, want %q", c.env, got, c.want)
		}
	}
}

func TestDetectReturnsSupportedLanguage(t *testing.T) {
	if got := Detect(); !IsSupported(got) {
		t.Errorf("Detect() = %q, not a supported language", got)
	}
}

func TestMustLoadPanicsOnBadJSON(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("expected panic for invalid JSON")
		}
	}()
	mustLoad(fstest.MapFS{"locales/en/x.json": {Data: []byte("{")}})
}

// namespaces 列出某語言目前存在的命名空間檔（engine/report/ui 由其他任務陸續加入，缺哪個都不報錯）。
func namespaces(t *testing.T, lang string) map[string]map[string]string {
	t.Helper()
	files, err := fs.Glob(localesFS, path.Join("locales", lang, "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]map[string]string{}
	for _, f := range files {
		raw, err := fs.ReadFile(localesFS, f)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]string
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		out[strings.TrimSuffix(path.Base(f), ".json")] = m
	}
	return out
}

func placeholders(s string) []string {
	var out []string
	for _, m := range placeholder.FindAllStringSubmatch(s, -1) {
		out = append(out, m[1])
	}
	sort.Strings(out)
	return out
}

func TestLocaleFilesConsistent(t *testing.T) {
	all := map[string]map[string]map[string]string{}
	nsSet := map[string]bool{}
	for _, lang := range Languages {
		all[lang] = namespaces(t, lang)
		for ns := range all[lang] {
			nsSet[ns] = true
		}
	}
	if !nsSet["cli"] {
		t.Fatal("cli namespace missing")
	}
	for ns := range nsSet {
		for _, lang := range Languages {
			if _, ok := all[lang][ns]; !ok {
				t.Errorf("namespace %s missing for %s", ns, lang)
			}
		}
		base := all[En][ns]
		for _, lang := range Languages {
			cur := all[lang][ns]
			for k, v := range cur {
				if !strings.HasPrefix(k, ns+".") {
					t.Errorf("%s/%s: key %q lacks prefix %q", lang, ns, k, ns+".")
				}
				if strings.TrimSpace(v) == "" {
					t.Errorf("%s/%s: key %q is empty", lang, ns, k)
				}
				if _, ok := base[k]; !ok {
					t.Errorf("%s/%s: key %q not in en", lang, ns, k)
					continue
				}
				if a, b := placeholders(v), placeholders(base[k]); strings.Join(a, ",") != strings.Join(b, ",") {
					t.Errorf("%s/%s: key %q placeholders %v != en %v", lang, ns, k, a, b)
				}
			}
			for k := range base {
				if _, ok := cur[k]; !ok {
					t.Errorf("%s/%s: key %q missing", lang, ns, k)
				}
			}
		}
	}
}

func TestNoKeyDefinedInTwoNamespaces(t *testing.T) {
	for _, lang := range Languages {
		seen := map[string]string{}
		for ns, m := range namespaces(t, lang) {
			for k := range m {
				if prev, dup := seen[k]; dup {
					t.Errorf("%s: key %q in both %s and %s", lang, k, prev, ns)
				}
				seen[k] = ns
			}
		}
	}
}

// scriptPairs 是常用的繁簡差異字（每組兩個字元：繁體、簡體）。
// 已排除兩種書寫系統都正當使用的字（只、后、里、台、制、准、周、复、郁、干…）。
const scriptPairs = "資资夾夹預预設设環环參参數数誤误範范圍围檔档無无開开輸输錄录來来節节點点請请動动認认識识讀读個个產产體体選选語语頁页標标題题錯错說说為为對对應应與与這这時时間间會会後后過过還还從从長长門门問问關关現现見见觀观親亲學学習习書书買买賣卖車车軟软輕轻較较載载轉转達达運运遠远連连進进邊边們们傳传備备價价優优兩两內内農农決决況况凍冻減减則则剛刚創创劍剑勞劳務务勝胜區区協协單单衛卫廠厂廣广張张彈弹強强歸归當当態态懷怀戰战戲戏戶户擇择據据擴扩擁拥擔担敗败敵敌斷断條条樣样檢检構构樂乐機机權权歡欢歷历殺杀氣气漢汉湯汤溝沟滿满滾滚漸渐澤泽濟济濕湿灣湾烏乌熱热獨独獲获獎奖畫画異异發发盡尽監监盤盘礎础禮礼離离種种稱称穩稳窮穷競竞筆笔籤签紅红純纯紙纸級级紀纪約约紹绍終终組组細细經经結结給给絕绝統统絡络續续維维綜综綠绿網网緊紧線线編编縮缩總总績绩繞绕繼继纖纤罰罚罷罢羅罗聖圣聯联聲声職职聽听脅胁腦脑興兴舉举艱艰蘭兰處处號号蟲虫術术裝装補补視视覽览覺觉觸触計计訂订訊讯討讨訓训託托記记許许訪访評评詞词詢询試试詳详話话該该課课誰谁調调談谈論论諸诸謝谢證证譯译護护變变讓让豐丰貝贝負负財财責责貨货貴贵費费賬账質质購购贈赠贏赢趕赶軌轨輔辅輪轮輯辑辦办辭辞適适遲迟遺遗邏逻郵邮鄉乡醫医釋释針针鈴铃鋼钢錢钱鍵键鎖锁鏡镜鐘钟鐵铁閉闭閱阅闊阔階阶際际陽阳陰阴陣阵陸陆隊队隨随險险隱隐雖虽雙双雜杂難难電电響响頂顶順顺須须頭头頻频顆颗願愿類类顯显風风飛飞飲饮餘余館馆馬马驅驱驗验驚惊髮发鬥斗魚鱼鮮鲜鳥鸟鳴鸣鷹鹰麗丽麥麦黃黄黨党齊齐齡龄龍龙龜龟傷伤兒儿償偿專专尋寻導导屬属層层嶼屿幣币廳厅悅悦愛爱慶庆憂忧憑凭懶懒擬拟攝摄暫暂淚泪淺浅測测溫温滅灭潔洁潛潜濃浓煙烟爭争牆墙犧牺狀状猶犹獸兽瑪玛畢毕疇畴療疗瘋疯眾众矯矫礦矿碼码確确稅税積积簡简簽签籌筹糧粮紐纽絲丝綁绑縣县繪绘纜缆聰聪肅肃脈脉腸肠膚肤臉脸艦舰薦荐藝艺藥药蘇苏虛虚蝦虾衝冲襲袭訴诉診诊豬猪貓猫貿贸賠赔賴赖趨趋蹤踪軍军遞递遷迁鄰邻釘钉鉤钩銀银銷销鋪铺鑰钥閃闪閒闲陳陈陝陕"

func scriptSets(t *testing.T) (trad, simp string) {
	t.Helper()
	runes := []rune(scriptPairs)
	if len(runes)%2 != 0 || len(runes)/2 < 200 {
		t.Fatalf("scriptPairs must hold at least 200 pairs, has %d runes", len(runes))
	}
	var tb, sb strings.Builder
	for i := 0; i < len(runes); i += 2 {
		if runes[i] == runes[i+1] {
			continue
		}
		tb.WriteRune(runes[i])
		sb.WriteRune(runes[i+1])
	}
	return tb.String(), sb.String()
}

func TestScriptHygiene(t *testing.T) {
	trad, simp := scriptSets(t)
	if n := len([]rune(trad)); n < 200 {
		t.Fatalf("only %d differing pairs", n)
	}
	isHan := func(r rune) bool { return unicode.Is(unicode.Han, r) }
	for ns, m := range namespaces(t, ZhHans) {
		for k, v := range m {
			if i := strings.IndexAny(v, trad); i >= 0 {
				t.Errorf("zh-Hans/%s %q contains traditional character %q", ns, k, []rune(v[i:])[0])
			}
		}
	}
	for ns, m := range namespaces(t, ZhHant) {
		for k, v := range m {
			if i := strings.IndexAny(v, simp); i >= 0 {
				t.Errorf("zh-Hant/%s %q contains simplified character %q", ns, k, []rune(v[i:])[0])
			}
		}
	}
	for ns, m := range namespaces(t, En) {
		for k, v := range m {
			if strings.IndexFunc(v, isHan) >= 0 {
				t.Errorf("en/%s %q contains Han characters", ns, k)
			}
		}
	}
}

type goldenEntry struct {
	Key  string            `json:"key"`
	Args map[string]string `json:"args"`
	Want string            `json:"want"`
}

// golden 由 testdata/gen_cli_golden.py 直接執行 Python 原版產生；zh-Hant 必須與原文逐字相同。
func TestZhHantCLIMatchesPythonOriginal(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "cli_zh_hant_golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	var entries []goldenEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatal(err)
	}
	covered := map[string]bool{}
	for _, e := range entries {
		covered[e.Key] = true
		if got := T(ZhHant, e.Key, e.Args); got != e.Want {
			t.Errorf("%s:\n got  %q\n want %q", e.Key, got, e.Want)
		}
	}
	// 每個出自 Python 的 key 都必須有 golden；Go 版新增的 key 例外。
	goOnly := regexp.MustCompile(`^cli\.(help\.(lang|top|check_ranges|init_ranges)|error\.(hands_required|lang_invalid|top_range|mode_conflict|unknown_flag|missing_value|init_failed|no_home)|ranges_ignored|check\..+|init\..+)$`)
	for k := range catalogs[ZhHant] {
		if strings.HasPrefix(k, "cli.") && !covered[k] && !goOnly.MatchString(k) {
			t.Errorf("cli key %q has no Python golden entry and is not a Go-only key", k)
		}
	}
}
