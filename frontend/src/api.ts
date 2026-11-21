// 綁定契約 v1（見計畫「GUI 綁定契約 v1」）。後端合併後，把檔尾的 `api` 改接真正的 bindings。

export type Lang = "zh-Hant" | "zh-Hans" | "en";

export interface RangesState { path: string; exists: boolean; picked: boolean }
export interface State {
  lang: Lang;
  langSource: "saved" | "system";
  handsDir: string;
  ranges: RangesState;
  hero: string;
  topN: number;
  version: string;
  hasReport: boolean;
  busy: boolean;
  /** 設定檔損壞或欄位無效而改用預設值時的提示（後端已翻好；空＝無）。 */
  settingsWarning: string;
}

/** error 為 Go 端翻好的字串，空字串＝無錯誤。取消對話框是 {ok:false, cancelled:true}，不是錯誤。 */
export interface Result { ok: boolean; cancelled?: boolean; error?: string; path?: string }

/** base＝缺的底範圍檔名（含 .txt），沒缺時為空字串。 */
export interface MissingCell { cell: string; base: string; actions: string[] }
export interface MissingNode { node: string; nodeLabel: string; cells: MissingCell[] }
export interface MissingAction { text: string; names: string[] }
export interface Unreadable { name: string; reason: string }
export interface CheckView {
  ok: boolean;
  error?: string;
  slotsFilled: number;
  slotsTotal: number;
  mapped: number;
  standardCount: number;
  complete: boolean;
  missingNodes: MissingNode[];
  missingActions: MissingAction[];
  unfilledRedundant: string[];
  unfilledMissing: string[];
  ignored: string[];
  duplicates: string[];
  unreadable: Unreadable[];
}

export interface InitView {
  ok: boolean;
  cancelled?: boolean;
  error?: string;
  dir: string;
  created: number;
  skipped: number;
  /** 因為已有對應檔（existing）而沒有建立的樣板檔（name）。 */
  mapped: { name: string; existing: string }[];
  howTo: "created" | "exists" | "";
}

/** 數字已在 Go 端格式化成字串。 */
export interface DevRow {
  spot: string; node: string; action: string;
  obs: string; theory: string; diff: string; n: string; z: string;
  clamped: boolean;
  anchorID: string;
}
export interface Deviations { pool: DevRow[]; me: DevRow[] }
export interface ReportView {
  ok: boolean;
  error?: string;
  summary: string[];
  warnings: string[];
  notice: string;
  deviations: Deviations;
  reportURL: string;
}

export interface Api {
  getState(): Promise<State>;
  setLanguage(lang: Lang): Promise<Result>;
  setHero(name: string): Promise<Result>;
  setTopN(n: number): Promise<Result>;
  pickHandsDir(): Promise<Result>;
  pickRangesDir(): Promise<Result>;
  clearRangesDir(): Promise<Result>;
  checkRanges(): Promise<CheckView>;
  initRangeTemplate(): Promise<InitView>;
  generate(): Promise<ReportView>;
  getReportView(): Promise<ReportView>;
  saveHTML(): Promise<Result>;
  print(): Promise<Result>;
  openInBrowser(): Promise<Result>;
  messages(lang: Lang): Promise<Record<string, string>>;
}

/**
 * 正式建置只用 real。mock 在 frontend/dev（不在 src，正式 typecheck 看不到），
 * 只在 vite dev 且 VITE_MOCK=1（npm run dev:mock）時以 import.meta.glob 載入，不進 dist。
 */
export async function loadApi(): Promise<Api> {
  if (import.meta.env.DEV && import.meta.env.VITE_MOCK === "1") {
    const mods = import.meta.glob<{ mockApi: Api }>("../dev/mock.ts");
    return (await mods["../dev/mock.ts"]()).mockApi;
  }
  return (await import("./real")).realApi;
}
