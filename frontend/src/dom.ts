export const $ = <T extends HTMLElement = HTMLElement>(id: string): T => {
  const e = document.getElementById(id);
  if (!e) throw new Error("missing #" + id);
  return e as T;
};

type Attrs = Record<string, string | boolean | undefined>;

/** 建立元素；文字一律走 textContent（資料來自檔名與玩家名，不得進 innerHTML）。 */
export function el<K extends keyof HTMLElementTagNameMap>(
  tag: K, attrs: Attrs = {}, ...kids: (Node | string | null | undefined | false)[]
): HTMLElementTagNameMap[K] {
  const e = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs)) {
    if (v === undefined || v === false) continue;
    if (k === "class") e.className = String(v);
    else e.setAttribute(k, v === true ? "" : v);
  }
  for (const kid of kids) if (kid) e.append(kid);
  return e;
}

export const clear = (e: HTMLElement) => e.replaceChildren();
