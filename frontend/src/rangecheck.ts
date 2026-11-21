import type { CheckView } from "./api";
import { $, clear, el } from "./dom";
import { t } from "./i18n";

const open = new Set<string>(); // 各區的展開狀態；重繪（切語言、重新檢查）時保留

/** 展開／收合只切換 hidden 與 aria-expanded，不重繪，焦點不會掉。 */
function section(id: string, title: string, count: number, body: HTMLElement): HTMLElement {
  const bid = "cs-" + id;
  const head = el("button", { type: "button", class: "cs-head", "aria-expanded": String(open.has(id)), "aria-controls": bid },
    el("span", { class: "chev", "aria-hidden": "true" }), el("span", {}, title), el("span", { class: "cs-count" }, String(count)));
  body.id = bid; body.className = "cs-body"; body.hidden = !open.has(id);
  head.addEventListener("click", () => {
    const o = !open.has(id);
    if (o) open.add(id); else open.delete(id);
    head.setAttribute("aria-expanded", String(o)); body.hidden = !o;
  });
  return el("div", { class: "cs" }, head, body);
}

const list = (items: string[]) => el("ul", {}, ...items.map((s) => el("li", {}, s)));

/** 渲染檢查結果；view 為 null 或 ok:false 時只清空，摘要文字由呼叫端決定。 */
export function renderCheck(view: CheckView | null) {
  const sum = $("check-sum");
  const badge = $("check-badge");
  const secs = $("check-sections");
  clear(secs); sum.className = "check-sum";
  if (!view || !view.ok) { badge.hidden = true; return; }

  sum.textContent = t("ui.check.summary", { filled: view.slotsFilled, total: view.slotsTotal, mapped: view.mapped, standard: view.standardCount });
  badge.hidden = false;
  badge.className = "badge " + (view.complete ? "ok" : "no");
  badge.textContent = t(view.complete ? "ui.check.complete" : "ui.check.incomplete");

  const nodes = el("div", {}, ...view.missingNodes.map((n) => el("div", {},
    el("div", { class: "node-h" }, n.nodeLabel),
    ...n.cells.map((c) => el("div", { class: "cell-row" },
      el("span", {}, c.cell),
      c.base ? el("span", { class: "chip base" }, t("ui.check.baseNeeded", { name: c.base })) : null,
      ...c.actions.map((a) => el("span", { class: "chip" }, a)))))));
  const acts = el("ul", {}, ...view.missingActions.map((m) => el("li", {}, m.text, ...m.names.map((n) => el("span", { class: "why" }, n)))));
  const unread = el("ul", {}, ...view.unreadable.map((u) => el("li", {}, u.name, el("span", { class: "why" }, u.reason))));

  const parts: [string, string, number, HTMLElement][] = [
    ["nodes", t("ui.check.missingNodes"), view.missingNodes.length, nodes],
    ["acts", t("ui.check.missingActions"), view.missingActions.length, acts],
    ["um", t("ui.check.unfilledMissing"), view.unfilledMissing.length, list(view.unfilledMissing)],
    ["ur", t("ui.check.unfilledRedundant"), view.unfilledRedundant.length, list(view.unfilledRedundant)],
    ["ign", t("ui.check.ignored"), view.ignored.length, list(view.ignored)],
    ["dup", t("ui.check.duplicates"), view.duplicates.length, list(view.duplicates)],
    ["unr", t("ui.check.unreadable"), view.unreadable.length, unread],
  ];
  const shown = parts.filter((p) => p[2] > 0);
  if (shown.length === 0) secs.append(el("p", { class: "hint" }, t("ui.check.allGood")));
  for (const [id, title, n, body] of shown) secs.append(section(id, title, n, body));
}
