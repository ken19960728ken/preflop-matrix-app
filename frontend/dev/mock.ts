// 開發用假後端：模擬延遲、錯誤、取消。URL 參數控制情境：
//   ?lang=zh-Hans|en  ?report=1（啟動即有報告）  ?ranges=ok|missing|none  ?check=complete
//   ?fail=generate|hero|check|pick  ?cancel=1（選資料夾回傳取消）  ?warn=0（無警告）|many（6 條）
import type { Api, CheckView, DevRow, InitView, Lang, ReportView, Result, State } from "../src/api";
import zhHant from "../../internal/i18n/locales/zh-Hant/ui.json";
import zhHans from "../../internal/i18n/locales/zh-Hans/ui.json";
import en from "../../internal/i18n/locales/en/ui.json";
import D from "./testdata/mock-data.json"; // 位置名稱屬資料，不得寫在 .ts（internal/guard 掃描）

const q = new URLSearchParams(location.search);
const UI: Record<Lang, Record<string, string>> = { "zh-Hant": zhHant, "zh-Hans": zhHans, en };
const sleep = (ms: number) => new Promise<void>((r) => setTimeout(r, ms));
const isLang = (s: string | null): s is Lang => s === "zh-Hant" || s === "zh-Hans" || s === "en";

// Go 端會翻好的字串；mock 只備三語各一份。
const SRV: Record<Lang, Record<string, string>> = {
  "zh-Hant": {
    noHands: "請先選擇手牌資料夾。", fail: "無法讀取手牌資料夾：權限不足。", hero: "Hero 名稱不可為空。", check: "找不到範圍資料夾。",
    s1: "共 12,480 手，其中 Hero 3,212 手。", s2: "掃描 48 個檔案，略過 2 個非手牌檔。",
    w1: "有 37 手的玩家名稱無法對應，已略過。", w2: "理論範圍不完整：缺少 6 個必要動作。",
    notice: "理論範圍不完整，缺的格子不顯示理論值。", saved: "/Users/you/Documents/preflop_matrix.html",
    settings: "設定檔無法讀取，已改用預設值。", nodeOpen: "開池", node3: "對 3-bet", node4: "對 4-bet", base: "需要先有", ua: "無法解析：第 3 行", dup: "重複",
  },
  "zh-Hans": {
    noHands: "请先选择手牌文件夹。", fail: "无法读取手牌文件夹：权限不足。", hero: "Hero 名称不能为空。", check: "找不到范围文件夹。",
    s1: "共 12,480 手，其中 Hero 3,212 手。", s2: "扫描 48 个文件，跳过 2 个非手牌文件。",
    w1: "有 37 手的玩家名称无法对应，已跳过。", w2: "理论范围不完整：缺少 6 个必要动作。",
    notice: "理论范围不完整，缺少的格子不显示理论值。", saved: "/Users/you/Documents/preflop_matrix.html",
    settings: "设置文件无法读取，已改用默认值。", nodeOpen: "开池", node3: "对 3-bet", node4: "对 4-bet", base: "需要先有", ua: "无法解析：第 3 行", dup: "重复",
  },
  en: {
    noHands: "Choose a hand history folder first.", fail: "Cannot read the hand folder: permission denied.", hero: "Hero name cannot be empty.", check: "Ranges folder not found.",
    s1: "12,480 hands, 3,212 of them Hero's.", s2: "Scanned 48 files, skipped 2 non-hand files.",
    w1: "37 hands had player names that could not be matched and were skipped.", w2: "Theory ranges are incomplete: 6 required actions missing.",
    notice: "Theory ranges are incomplete; cells without them show no theory value.", saved: "/Users/you/Documents/preflop_matrix.html",
    settings: "The settings file could not be read; defaults are in use.", nodeOpen: "Open", node3: "vs 3-bet", node4: "vs 4-bet", base: "needed first", ua: "cannot parse: line 3", dup: "duplicate",
  },
};

const s0 = q.get("lang");
let lang: Lang = isLang(s0) ? s0 : "zh-Hant";
let handsDir = q.get("report") ? "/Users/you/Poker/GG/HandHistory" : "";
let rangesMode = q.get("ranges") ?? (q.get("report") ? "ok" : "none");
let hero = "Hero";
let topN = 8;
let hasReport = q.get("report") === "1";
let version = 1;
const rangesPath = () => (rangesMode === "none" ? "" : "/Users/you/Poker/Ranges/RushCash");
const sv = () => SRV[lang];

const rows = (): DevRow[] => {
  const names = [sv().nodeOpen, sv().node3, sv().node4];
  return D.rows.slice(0, topN).map(([spot, ni, act, obs, th, diff, n, z, cl]) => {
    const [a, , b] = String(spot).split(" ");
    return { spot: String(spot), node: names[Number(ni)], action: String(act), obs: String(obs), theory: String(th), diff: String(diff), n: String(n), z: String(z), clamped: Boolean(cl), anchorID: `cell-vs_3bet-${a}-${b}-pool-r` };
  });
};

const reportView = (): ReportView => {
  const warn = q.get("warn") !== "0";
  const many = q.get("warn") === "many";
  const incomplete = rangesMode === "ok" && q.get("check") !== "complete";
  return {
    ok: true,
    summary: [sv().s1, sv().s2],
    warnings: !warn ? [] : many ? [sv().w1, sv().w2, sv().w1, sv().w2, sv().w1, sv().w2] : [sv().w1, sv().w2],
    notice: incomplete || rangesMode === "none" ? sv().notice : "",
    deviations: { pool: rows(), me: rows().filter((_, i) => i % 2 === 0) },
    reportURL: `/report/current.html?embed=1&lang=${lang}&v=${version}`,
  };
};

const checkView = (): CheckView => {
  const complete = q.get("check") === "complete";
  const base = {
    ok: true, mapped: 102, standardCount: 108, complete,
    slotsFilled: complete ? 95 : 89, slotsTotal: 95,
    missingNodes: [] as CheckView["missingNodes"], missingActions: [] as CheckView["missingActions"],
    unfilledRedundant: [] as string[], unfilledMissing: [] as string[], ignored: [] as string[],
    duplicates: [] as string[], unreadable: [] as CheckView["unreadable"],
  };
  if (complete) return { ...base, ignored: ["notes.txt"] };
  const names = [sv().nodeOpen, sv().node3, sv().node4];
  return {
    ...base,
    missingNodes: D.missingNodes.map((n) => ({ node: n.node, nodeLabel: names[n.nodeIdx], cells: n.cells.map((c) => ({ ...c, base: c.base ? D.dupOf : "" })) })),
    missingActions: D.missingActions,
    unfilledMissing: D.unfilledMissing,
    unfilledRedundant: D.unfilledRedundant,
    ignored: ["notes.txt", "old/draft.txt"],
    duplicates: [sv().dup + "：" + D.dupOf],
    unreadable: [{ name: D.unreadableName, reason: sv().ua }],
  };
};

const fail = (): Result => ({ ok: false, error: sv().fail });
const unusable = (): Result => ({ ok: false, error: sv().hero });

export const mockApi: Api = {
  async getState(): Promise<State> {
    await sleep(120);
    return {
      lang, langSource: "system", handsDir,
      ranges: { path: rangesPath(), exists: rangesMode !== "missing", picked: rangesMode !== "none" },
      hero, topN, version: "0.1.0-mock", hasReport, busy: false,
      settingsWarning: q.get("warnsettings") === "1" ? sv().settings : "",
    };
  },
  async setLanguage(l) { await sleep(60); lang = l; return { ok: true }; },
  async setHero(name) {
    await sleep(80);
    if (q.get("fail") === "hero" || !name.trim()) return unusable();
    hero = name.trim(); return { ok: true };
  },
  async setTopN(n) { await sleep(80); topN = n; version++; return { ok: true }; },
  async pickHandsDir() {
    await sleep(250);
    if (q.get("cancel") === "1") return { ok: false, cancelled: true };
    if (q.get("fail") === "pick") return fail();
    handsDir = "/Users/you/Poker/GG/HandHistory"; return { ok: true, path: handsDir };
  },
  async pickRangesDir() {
    await sleep(250);
    if (q.get("cancel") === "1") return { ok: false, cancelled: true };
    rangesMode = "ok"; return { ok: true, path: rangesPath() };
  },
  async clearRangesDir() { rangesMode = "none"; return { ok: true }; },
  async checkRanges() {
    await sleep(500);
    if (rangesMode === "none") return { ...checkView(), ok: false, error: UI[lang]["ui.check.noRange"] };
    if (rangesMode === "missing" || q.get("fail") === "check") return { ...checkView(), ok: false, error: sv().check };
    return checkView();
  },
  async initRangeTemplate(): Promise<InitView> {
    await sleep(700);
    const mapped = [{ name: D.unreadableName, existing: D.dupOf }, { name: D.dupOf, existing: D.unreadableName }];
    if (q.get("cancel") === "1") return { ok: false, cancelled: true, dir: "", created: 0, skipped: 0, mapped: [], howTo: "created" };
    return { ok: true, dir: rangesPath() || "/Users/you/Poker/Ranges/New", created: 6, skipped: 102, mapped, howTo: "created" };
  },
  async generate() {
    for (let i = 0; i < 12; i++) {
      await sleep(100);
    }
    if (!handsDir) return { ok: false, error: sv().noHands, summary: [], warnings: [], notice: "", deviations: { pool: [], me: [] }, reportURL: "" };
    if (q.get("fail") === "generate") return { ok: false, error: sv().fail, summary: [], warnings: [], notice: "", deviations: { pool: [], me: [] }, reportURL: "" };
    hasReport = true; version++;
    return reportView();
  },
  async getReportView() { await sleep(60); return reportView(); },
  async saveHTML() { await sleep(200); return q.get("cancel") === "1" ? { ok: false, cancelled: true } : { ok: true, path: sv().saved }; },
  async print() { await sleep(100); return { ok: true }; },
  async openInBrowser() { await sleep(100); return { ok: true }; },
  async messages(l) { return UI[l]; },
};
