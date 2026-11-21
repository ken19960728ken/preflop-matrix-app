// 真後端：把 Wails bindings 包成 Api。bindings 的 slice 是 T[] | null，這裡一律轉成陣列。
import { AppService } from "../bindings/github.com/ken19960728ken/preflop-matrix-app/internal/gui";
import type * as B from "../bindings/github.com/ken19960728ken/preflop-matrix-app/internal/gui";
import type { Api, CheckView, InitView, Lang, ReportView, Result, State } from "./api";

const arr = <T>(a: T[] | null | undefined): T[] => a ?? [];

const result = (r: B.Result): Result => ({ ok: r.ok, cancelled: r.cancelled, error: r.error, path: r.path });

const report = (r: B.ReportView): ReportView => ({
  ok: r.ok, error: r.error, notice: r.notice, reportURL: r.reportURL,
  summary: arr(r.summary), warnings: arr(r.warnings),
  deviations: { pool: arr(r.deviations?.pool), me: arr(r.deviations?.me) },
});

const check = (c: B.CheckView): CheckView => ({
  ok: c.ok, error: c.error, slotsFilled: c.slotsFilled, slotsTotal: c.slotsTotal, mapped: c.mapped,
  standardCount: c.standardCount, complete: c.complete,
  missingNodes: arr(c.missingNodes).map((n) => ({
    node: n.node, nodeLabel: n.nodeLabel,
    cells: arr(n.cells).map((x) => ({ cell: x.cell, base: x.base, actions: arr(x.actions) })),
  })),
  missingActions: arr(c.missingActions).map((m) => ({ text: m.text, names: arr(m.names) })),
  unfilledRedundant: arr(c.unfilledRedundant), unfilledMissing: arr(c.unfilledMissing),
  ignored: arr(c.ignored), duplicates: arr(c.duplicates),
  unreadable: arr(c.unreadable).map((u) => ({ name: u.name, reason: u.reason })),
});

const init = (v: B.InitView): InitView => ({
  ok: v.ok, cancelled: v.cancelled, error: v.error, dir: v.dir, created: v.created, skipped: v.skipped,
  mapped: arr(v.mapped).map((m) => ({ name: m.name, existing: m.existing })),
  howTo: v.howTo === "created" || v.howTo === "exists" ? v.howTo : "",
});

export const realApi: Api = {
  async getState(): Promise<State> {
    const s = await AppService.GetState();
    return { ...s, lang: s.lang as Lang, langSource: s.langSource === "saved" ? "saved" : "system" };
  },
  async setLanguage(lang) { return result(await AppService.SetLanguage(lang)); },
  async setHero(name) { return result(await AppService.SetHero(name)); },
  async setTopN(n) { return result(await AppService.SetTopN(n)); },
  async pickHandsDir() { return result(await AppService.PickHandsDir()); },
  async pickRangesDir() { return result(await AppService.PickRangesDir()); },
  async clearRangesDir() { return result(await AppService.ClearRangesDir()); },
  async checkRanges() { return check(await AppService.CheckRanges()); },
  async initRangeTemplate() { return init(await AppService.InitRangeTemplate()); },
  async generate() { return report(await AppService.Generate()); },
  async getReportView() { return report(await AppService.GetReportView()); },
  async saveHTML() { return result(await AppService.SaveHTML()); },
  async print() { return result(await AppService.Print()); },
  async openInBrowser() { return result(await AppService.OpenInBrowser()); },
  async messages(lang) {
    const m = (await AppService.Messages(lang)) ?? {};
    return Object.fromEntries(Object.entries(m).filter((e): e is [string, string] => typeof e[1] === "string"));
  },
};
