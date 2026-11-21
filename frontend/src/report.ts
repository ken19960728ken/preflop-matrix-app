import { $ } from "./dom";

const ANCHOR = /^cell-[A-Za-z0-9_-]+$/;
const REPORT_PREFIX = "/report/current.html?";

/** 報告 iframe：sandbox="allow-scripts"（不加 allow-same-origin，見 index.html）。 */
export function createReport() {
  const frame = $<HTMLIFrameElement>("frame");
  const loading = $("frame-loading");
  const empty = $("empty");
  let base = "";
  let loaded = false;
  let pending = "";

  const goto = (id: string) => {
    // query 必須與目前 src 一致，只換 hash。
    frame.src = base + "#" + id;
    frame.scrollIntoView({ block: "nearest" });
  };

  frame.addEventListener("load", () => {
    loaded = true;
    loading.hidden = true;
    if (pending) { const id = pending; pending = ""; goto(id); }
  });

  return {
    /** url 為空＝無報告，顯示空狀態。同一個 url 不重載。 */
    show(url: string) {
      if (!url || !url.startsWith(REPORT_PREFIX)) {
        base = ""; loaded = false; pending = "";
        frame.removeAttribute("src"); frame.hidden = true; loading.hidden = true; empty.hidden = false;
        return;
      }
      empty.hidden = true; frame.hidden = false;
      if (url === base) return;
      base = url; loaded = false; pending = ""; loading.hidden = false;
      frame.src = url;
    },
    jump(anchorID: string) {
      if (!base || !ANCHOR.test(anchorID)) return;
      if (!loaded) { pending = anchorID; return; }
      goto(anchorID);
    },
  };
}
