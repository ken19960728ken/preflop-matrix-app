import type { DevRow, Deviations } from "./api";
import { $, clear, el } from "./dom";
import { t } from "./i18n";

type Which = "pool" | "me";

const COLS = ["spot", "node", "action", "obs", "theory", "diff", "n", "z"] as const;
const signClass = (v: string) => (/^[+]/.test(v) ? "up" : /^[−-]/.test(v) ? "down" : "");

export function createDeviations(onJump: (anchorID: string) => void, topN: () => number) {
  const panel = $("dev-panel");
  const tabs = Array.from(document.querySelectorAll<HTMLButtonElement>("#dev-tabs [role=tab]"));
  let data: Deviations = { pool: [], me: [] };
  let which: Which = "pool";
  let selected = "";

  const row = (r: DevRow): HTMLTableRowElement => {
    const jump = () => { selected = r.anchorID; mark(); onJump(r.anchorID); };
    const spot = el("button", { type: "button", class: "spot" }, r.spot);
    spot.addEventListener("click", (e) => { e.stopPropagation(); jump(); });
    const theory = el("td", {}, r.theory);
    if (r.clamped) {
      const note = t("ui.dev.clampedNote");
      theory.append(el("span", { class: "clamp", title: note, tabindex: "0", role: "note", "aria-label": t("ui.dev.clamped") + "：" + note }, t("ui.dev.clamped")));
    }
    const tr = el("tr", { "data-anchor": r.anchorID },
      el("td", { class: "l" }, spot), el("td", { class: "l" }, r.node), el("td", { class: "l" }, r.action),
      el("td", {}, r.obs), theory,
      el("td", { class: signClass(r.diff) }, r.diff), el("td", {}, r.n), el("td", { class: signClass(r.z) }, r.z));
    tr.addEventListener("click", jump);
    return tr;
  };

  const mark = () => panel.querySelectorAll("tbody tr").forEach((tr) => tr.classList.toggle("sel", tr.getAttribute("data-anchor") === selected));

  const render = () => {
    clear(panel);
    const rows = data[which];
    if (rows.length === 0) { panel.append(el("p", { class: "dev-empty" }, t(topN() === 0 ? "ui.dev.emptyZero" : "ui.dev.empty"))); return; }
    const head = el("tr", {}, ...COLS.map((c, i) => el("th", { scope: "col", class: i < 3 ? "l" : "" }, t("ui.dev.col." + c))));
    panel.append(el("table", { class: "t" }, el("thead", {}, head), el("tbody", {}, ...rows.map(row))));
    mark();
  };

  const select = (w: Which, focus = false) => {
    which = w; selected = "";
    tabs.forEach((b) => { const on = b.dataset.which === w; b.setAttribute("aria-selected", String(on)); b.tabIndex = on ? 0 : -1; if (on && focus) b.focus(); });
    render();
  };
  tabs.forEach((b, i) => {
    b.addEventListener("click", () => select(b.dataset.which as Which));
    b.addEventListener("keydown", (e) => {
      if (e.key !== "ArrowRight" && e.key !== "ArrowLeft") return;
      e.preventDefault();
      select(tabs[(i + 1) % tabs.length].dataset.which as Which, true);
    });
  });

  return { set(d: Deviations) { data = d; selected = ""; render(); }, render };
}
