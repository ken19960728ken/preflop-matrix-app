import "./style.css";
import { loadApi } from "./api";
import type { Api, CheckView, Lang, ReportView, Result, State } from "./api";
import { $, clear, el } from "./dom";
import { applyStatic, setDict, t } from "./i18n";
import { createDeviations } from "./deviations";
import { createReport } from "./report";
import { renderCheck } from "./rangecheck";

const WARN_COLLAPSE_OVER = 3; // 警告超過此數預設收合
const SHORT_WINDOW = 860; // 視窗高度（px）不超過此值時，偏離抽屜預設收合
const DEV_OPEN_KEY = "pma.devOpen";
const TOPN_MAX = 20;
const POLL_MS = 1000;

const EMPTY_CHECK: CheckView = {
  ok: false, slotsFilled: 0, slotsTotal: 0, mapped: 0, standardCount: 0, complete: false, missingNodes: [], missingActions: [],
  unfilledRedundant: [], unfilledMissing: [], ignored: [], duplicates: [], unreadable: [],
};

let api: Api;
let state: State;
let view: ReportView | null = null;
let check: CheckView | null = null;
let checking = false;
let checkSeq = 0;
let busy = false;
let templating = false;
let langBusy = false;
let topnBusy = false;
let warnsOpen = false;
let heroSaved = "";
let heroOp: Promise<void> | null = null;
let ticker = 0;

const report = createReport();
const dev = createDeviations((id) => report.jump(id), () => state.topN);
const input = (id: string) => $(id) as HTMLInputElement;

// ---------- 小工具 ----------
const msgOf = (e: unknown) => t("ui.error.unexpected", { msg: e instanceof Error ? e.message : String(e) });
const unexpected = () => t("ui.error.unexpected", { msg: "?" });

function setErr(id: string, msg?: string) {
  const e = $(id);
  e.textContent = msg ?? ""; e.hidden = !msg;
}

/** 呼叫後端；擲出的（非預期）錯誤轉成行內訊息。預期失敗在 Result.error。 */
async function call<T>(errId: string, fn: () => Promise<T>): Promise<T | undefined> {
  try { return await fn(); } catch (e) { setErr(errId, msgOf(e)); return undefined; }
}

/** 回傳 true＝不必繼續（失敗或取消）。取消對話框不顯示錯誤。 */
function failed(r: Result | undefined, errId: string): boolean {
  if (!r || r.cancelled) return true;
  if (!r.ok) { setErr(errId, r.error || unexpected()); return true; }
  return false;
}

async function refreshState() {
  const s = await call("gen-err", () => api.getState());
  if (s) state = s;
}

async function reloadReportView() {
  const r = await call("gen-err", () => api.getReportView());
  if (!r) return;
  if (r.ok) view = r; else setErr("gen-err", r.error || unexpected());
}

// ---------- 渲染 ----------
function updateDisabled() {
  const lock = busy || templating;
  const set = (id: string, off: boolean) => $(id).toggleAttribute("disabled", off);
  set("generate", busy);
  for (const id of ["hands-pick", "ranges-pick", "ranges-clear", "tpl-btn", "hero"]) set(id, lock);
  set("check-recheck", lock || checking || !state.ranges.picked);
  for (const id of ["act-save", "act-print", "act-browser"]) set(id, !state.hasReport || busy);
  document.querySelectorAll<HTMLButtonElement>("#langsw button").forEach((b) => { b.disabled = langBusy; });
}

function renderBusy() {
  $("generate").setAttribute("aria-busy", String(busy));
  $("busy").hidden = !busy;
  updateDisabled();
}

function renderPaths() {
  const hp = $("hands-path");
  hp.textContent = state.handsDir ? "‎" + state.handsDir : t("ui.hands.none");
  hp.title = state.handsDir; hp.classList.toggle("none", !state.handsDir);

  const { picked, exists, path } = state.ranges;
  const bad = picked && !exists;
  const rp = $("ranges-path");
  rp.textContent = picked ? "‎" + path : t("ui.ranges.none");
  rp.title = picked ? path : ""; rp.classList.toggle("none", !picked); rp.classList.toggle("bad", bad);
  $("ranges-status").hidden = !bad; $("ranges-status").textContent = t("ui.ranges.notFound");
  $("ranges-extra").hidden = !picked;
  $("ranges-pick").textContent = t(bad ? "ui.ranges.repick" : "ui.pick");
  updateDisabled();
}

function renderCheckPanel() {
  const sum = $("check-sum");
  const plain = (msg: string, err = false) => { renderCheck(null); sum.textContent = msg; sum.className = "check-sum plain" + (err ? " inline-err" : ""); };
  if (checking) return plain(t("ui.check.running"));
  if (!state.ranges.picked) return plain(t("ui.check.noRange"));
  if (!check) return plain("");
  if (!check.ok) return plain(check.error ?? "", true);
  renderCheck(check);
}

function renderWarnings(warnings: string[]) {
  const box = $("warns"); const toggle = $("warns-toggle"); const list = $("warns-list");
  box.hidden = warnings.length === 0;
  $("warns-h").textContent = t("ui.warnings.title", { n: warnings.length });
  clear(list);
  warnings.forEach((w) => list.append(el("li", {}, w)));
  const collapsible = warnings.length > WARN_COLLAPSE_OVER;
  toggle.hidden = !collapsible;
  list.hidden = collapsible && !warnsOpen;
  toggle.setAttribute("aria-expanded", String(!list.hidden));
  toggle.textContent = t(list.hidden ? "ui.warnings.showAll" : "ui.warnings.collapse", { n: warnings.length });
}

function renderReport() {
  const has = state.hasReport && !!view?.ok;
  report.show(has && view ? view.reportURL : "");
  $("msgs").hidden = !has; $("dev").hidden = !has;
  updateDisabled();
  if (!has || !view) return;
  const sumEl = $("summary"); clear(sumEl);
  view.summary.forEach((s) => sumEl.append(el("li", {}, s)));
  sumEl.setAttribute("aria-label", t("ui.summary.title"));
  const notice = $("notice"); clear(notice);
  notice.hidden = !view.notice;
  if (view.notice) notice.append(el("strong", {}, t("ui.notice.title")), el("span", {}, view.notice));
  renderWarnings(view.warnings);
  dev.set(view.deviations);
}

function syncChrome() {
  document.documentElement.lang = state.lang;
  document.querySelectorAll<HTMLButtonElement>("#langsw button").forEach((b) => b.setAttribute("aria-pressed", String(b.dataset.lang === state.lang)));
  $("ver").textContent = state.version ? t("ui.version", { v: state.version }) : "";
  $("settings-warn").textContent = state.settingsWarning;
  $("settings-warn").hidden = !state.settingsWarning;
  syncSideToggle(); syncDevToggle();
}
function syncSideToggle() {
  const closed = $("app").classList.contains("side-closed");
  const b = $("side-toggle");
  const label = t(closed ? "ui.controls.expand" : "ui.controls.collapse");
  b.setAttribute("aria-expanded", String(!closed)); b.setAttribute("aria-label", label); b.title = label;
}
function syncDevToggle() {
  const b = $("dev-toggle");
  b.title = t(b.getAttribute("aria-expanded") === "true" ? "ui.dev.collapse" : "ui.dev.expand");
}
function setDevOpen(open: boolean) {
  $("dev-toggle").setAttribute("aria-expanded", String(open)); $("dev-body").hidden = !open; syncDevToggle();
}

function renderAll() {
  applyStatic(); syncChrome(); renderPaths(); renderBusy(); renderCheckPanel(); renderReport();
  input("hero").placeholder = "Hero";
}

// ---------- 產生報告 ----------
function startTicker() {
  const t0 = Date.now();
  const tick = () => { $("busy-sec").textContent = t("ui.generate.elapsed", { s: Math.floor((Date.now() - t0) / 1000) }); };
  tick(); ticker = window.setInterval(tick, POLL_MS);
  $("gen-live").textContent = t("ui.generate.busy");
}
function stopTicker(done: boolean) {
  window.clearInterval(ticker); ticker = 0;
  $("busy-sec").textContent = "";
  $("gen-live").textContent = done ? t("ui.generate.done") : "";
}

function commitHero(): Promise<void> {
  heroOp ??= doCommitHero().finally(() => { heroOp = null; });
  return heroOp;
}
async function doCommitHero() {
  const name = input("hero").value.trim();
  setErr("hero-err");
  if (name === heroSaved) return;
  const r = await call("hero-err", () => api.setHero(name));
  if (!r) return;
  if (!r.ok) { setErr("hero-err", r.error || unexpected()); return; }
  await refreshState();
  heroSaved = state.hero; input("hero").value = state.hero;
}
/** 產生前一定要把 Hero 存好（不依賴 blur）；存不進去就中止，hero-err 保留。 */
async function ensureHero(): Promise<boolean> {
  for (let i = 0; i < 2 && input("hero").value.trim() !== heroSaved; i++) await commitHero();
  return input("hero").value.trim() === heroSaved;
}

async function generate() {
  if (busy) return;
  busy = true; renderBusy();
  let ok = false;
  try {
    if (!(await ensureHero())) return;
    setErr("gen-err"); setErr("act-err"); $("act-msg").textContent = "";
    startTicker();
    const r = await call("gen-err", () => api.generate());
    if (r && !r.ok) setErr("gen-err", r.error || unexpected());
    await refreshState();
    if (r?.ok) { view = r; state = { ...state, hasReport: true }; ok = true; }
  } finally {
    stopTicker(ok);
    busy = false; renderBusy(); renderReport();
    $("generate").focus();
  }
}

/** 啟動時後端還在產生（例如視窗重載）：每秒輪詢到結束。 */
function watchBusy() {
  busy = true; renderBusy(); startTicker();
  const id = window.setInterval(async () => {
    const s = await call("gen-err", () => api.getState());
    if (!s || s.busy) return;
    window.clearInterval(id);
    state = s;
    if (state.hasReport) await reloadReportView();
    stopTicker(state.hasReport);
    busy = false; renderBusy(); renderReport();
  }, POLL_MS);
}

// ---------- 其他動作 ----------
async function runCheck() {
  const seq = ++checkSeq;
  if (!state.ranges.picked) { check = null; checking = false; renderCheckPanel(); updateDisabled(); return; }
  checking = true; renderCheckPanel(); updateDisabled();
  let res: CheckView;
  try { res = await api.checkRanges(); } catch (e) { res = { ...EMPTY_CHECK, error: msgOf(e) }; }
  if (seq !== checkSeq) return; // 已有更新的檢查
  check = res; checking = false; renderCheckPanel(); updateDisabled();
}

async function pickFolder(kind: "hands" | "ranges") {
  setErr("gen-err");
  const r = await call("gen-err", () => (kind === "hands" ? api.pickHandsDir() : api.pickRangesDir()));
  if (failed(r, "gen-err")) return;
  await refreshState(); renderPaths();
  if (kind === "ranges") { check = null; void runCheck(); }
}

async function clearRanges() {
  const r = await call("gen-err", () => api.clearRangesDir());
  if (failed(r, "gen-err")) return;
  checkSeq++; checking = false; check = null; $("tpl-res").hidden = true;
  await refreshState(); renderPaths(); renderCheckPanel();
}

async function runAction(fn: () => Promise<Result>, showPath = false) {
  setErr("act-err"); $("act-msg").textContent = "";
  const r = await call("act-err", fn);
  if (failed(r, "act-err")) return;
  if (showPath && r?.path) $("act-msg").textContent = t("ui.action.saved", { path: r.path });
}

async function makeTemplate() {
  setErr("tpl-err"); $("tpl-res").hidden = true;
  templating = true; $("tpl-status").hidden = false; updateDisabled();
  let r;
  try { r = await call("tpl-err", () => api.initRangeTemplate()); }
  finally { templating = false; $("tpl-status").hidden = true; updateDisabled(); }
  if (!r || r.cancelled) return;
  if (!r.ok) { setErr("tpl-err", r.error || unexpected()); return; }
  const res = $("tpl-res"); clear(res);
  res.append(
    el("span", { class: "dir" }, t("ui.template.dir", { dir: r.dir })),
    el("span", {}, t("ui.template.created", { created: r.created, skipped: r.skipped })),
    el("span", {}, t("ui.template.existing", { n: r.mapped.length })),
    el("span", { class: "hint" }, r.howTo ? t(r.howTo === "created" ? "ui.template.howTo.created" : "ui.template.howTo.exists") : ""));
  res.hidden = false;
  await refreshState();
  if (state.ranges.picked) { renderPaths(); void runCheck(); }
}

/** 切語言：先取齊新語言的文字、報告與狀態再一次套用；任何一步失敗就退回原語言。 */
async function setLang(lang: Lang) {
  if (langBusy || lang === state.lang) return;
  langBusy = true; updateDisabled();
  const prev = state.lang;
  try {
    const r = await api.setLanguage(lang);
    if (!r.ok) { setErr("gen-err", r.error || unexpected()); return; }
    try {
      const msgs = await api.messages(lang);
      const rv = state.hasReport ? await api.getReportView() : null;
      const st = await api.getState();
      setDict(msgs); state = st;
      if (rv?.ok) view = rv;
      for (const id of ["gen-err", "hero-err", "act-err", "tpl-err"]) setErr(id);
      if (rv && !rv.ok) setErr("gen-err", rv.error || unexpected());
      $("act-msg").textContent = ""; $("tpl-res").hidden = true;
      renderAll();
      if (state.ranges.picked) void runCheck(); // 檢查結果的字串由 Go 翻好，需重取
    } catch (e) {
      await api.setLanguage(prev).catch(() => undefined);
      throw e;
    }
  } catch (e) {
    setErr("gen-err", msgOf(e));
  } finally {
    langBusy = false; updateDisabled();
  }
}

async function commitTopN() {
  if (topnBusy) return;
  const el_ = input("topn");
  const n = Number(el_.value);
  if (el_.value.trim() === "" || !Number.isInteger(n) || n < 0 || n > TOPN_MAX) { setErr("topn-err", t("ui.dev.topNHint")); return; }
  setErr("topn-err");
  if (n === state.topN) return;
  topnBusy = true;
  try {
    const r = await call("topn-err", () => api.setTopN(n));
    if (!r) return;
    if (!r.ok) { setErr("topn-err", r.error || unexpected()); return; }
    state = { ...state, topN: n };
    if (state.hasReport) { await reloadReportView(); renderReport(); }
  } finally { topnBusy = false; }
}

// ---------- 啟動 ----------
function initialDevOpen(): boolean {
  try {
    const v = localStorage.getItem(DEV_OPEN_KEY);
    if (v === "1") return true;
    if (v === "0") return false;
  } catch { /* localStorage 不可用就用預設 */ }
  return window.innerHeight > SHORT_WINDOW;
}

function wire() {
  $("hands-pick").addEventListener("click", () => void pickFolder("hands"));
  $("ranges-pick").addEventListener("click", () => void pickFolder("ranges"));
  $("ranges-clear").addEventListener("click", () => void clearRanges());
  $("generate").addEventListener("click", () => void generate());
  $("tpl-btn").addEventListener("click", () => void makeTemplate());
  $("check-recheck").addEventListener("click", () => void runCheck());
  $("act-save").addEventListener("click", () => void runAction(() => api.saveHTML(), true));
  $("act-print").addEventListener("click", () => void runAction(() => api.print()));
  $("act-browser").addEventListener("click", () => void runAction(() => api.openInBrowser()));
  document.querySelectorAll<HTMLButtonElement>("#langsw button").forEach((b) => b.addEventListener("click", () => void setLang(b.dataset.lang as Lang)));

  const hero = input("hero");
  hero.addEventListener("blur", () => void commitHero());
  hero.addEventListener("keydown", (e) => { if (e.key === "Enter") { e.preventDefault(); void commitHero(); } });
  input("topn").addEventListener("change", () => void commitTopN());

  $("side-toggle").addEventListener("click", () => { $("app").classList.toggle("side-closed"); syncSideToggle(); });
  const ct = $("check-toggle");
  ct.addEventListener("click", () => {
    const o = ct.getAttribute("aria-expanded") !== "true";
    ct.setAttribute("aria-expanded", String(o)); $("check-body").hidden = !o;
  });
  $("dev-toggle").addEventListener("click", () => {
    const o = $("dev-toggle").getAttribute("aria-expanded") !== "true";
    setDevOpen(o);
    try { localStorage.setItem(DEV_OPEN_KEY, o ? "1" : "0"); } catch { /* 不可用就不記住 */ }
  });
  $("warns-toggle").addEventListener("click", () => { warnsOpen = !warnsOpen; if (view) renderWarnings(view.warnings); });
  setDevOpen(initialDevOpen());
}

async function main() {
  api = await loadApi();
  wire();
  state = await api.getState();
  setDict(await api.messages(state.lang));
  heroSaved = state.hero;
  input("hero").value = state.hero;
  input("topn").value = String(state.topN);
  if (state.hasReport) view = await api.getReportView();
  renderAll();
  document.body.classList.add("ready");
  if (state.busy) watchBusy();
  if (state.ranges.picked && state.ranges.exists) void runCheck();
}

main().catch((e) => { $("boot").textContent = e instanceof Error ? e.message : String(e); });
