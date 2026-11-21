let dict: Record<string, string> = {};

export function setDict(d: Record<string, string>) { dict = d; }

/** 缺 key 時退回 key 本身；{name} 佔位符代入。 */
export function t(key: string, args: Record<string, string | number> = {}): string {
  const s = Object.hasOwn(dict, key) ? dict[key] : key;
  return s.replace(/\{([A-Za-z0-9_]+)\}/g, (m, k) => (k in args ? String(args[k]) : m));
}

/** 套用靜態骨架上的 data-i（textContent）、data-i-title、data-i-label、data-i-ph。 */
export function applyStatic(root: ParentNode = document) {
  root.querySelectorAll<HTMLElement>("[data-i]").forEach((e) => { e.textContent = t(e.dataset.i!); });
  root.querySelectorAll<HTMLElement>("[data-i-title]").forEach((e) => { e.title = t(e.dataset.iTitle!); });
  root.querySelectorAll<HTMLElement>("[data-i-label]").forEach((e) => { e.setAttribute("aria-label", t(e.dataset.iLabel!)); });
}
