import { readFileSync, writeFileSync } from "node:fs";
import { defineConfig } from "vite";
import type { Plugin } from "vite";
import wails from "@wailsio/runtime/plugins/vite";

// vite 建置會清空 dist；補回 .gitkeep，讓乾淨 clone 的 go:embed all:frontend/dist 有檔可嵌。
const keepDistPlaceholder: Plugin = {
  name: "keep-dist-placeholder",
  apply: "build",
  closeBundle() {
    writeFileSync(new URL("./dist/.gitkeep", import.meta.url), "");
  },
};

// 只在 VITE_MOCK=1 的 dev server 掛上：以假報告頂替 Go 的 /report/current.html；不進 dist。
const serveMockReport: Plugin = {
  name: "serve-mock-report",
  apply: "serve",
  configureServer(server) {
    server.middlewares.use((req, res, next) => {
      if (!req.url?.startsWith("/report/current.html")) return next();
      res.setHeader("Content-Type", "text/html; charset=utf-8");
      res.end(readFileSync(new URL("./dev/testdata/mock-report.html", import.meta.url)));
    });
  },
};

// 主視窗 CSP（只在 build 注入，dev server 的 HMR 需要 ws 與 inline）。
const CSP = [
  "default-src 'self'",
  "script-src 'self'",
  "style-src 'self' https://fonts.googleapis.com",
  "font-src https://fonts.gstatic.com",
  "img-src 'self' data:",
  "connect-src 'self'",
  "frame-src 'self'",
  "object-src 'none'",
  "base-uri 'none'",
  "form-action 'none'",
].join("; ");
const injectCsp: Plugin = {
  name: "inject-csp",
  apply: "build",
  transformIndexHtml: () => [{ tag: "meta", attrs: { "http-equiv": "Content-Security-Policy", content: CSP }, injectTo: "head-prepend" }],
};

// https://vitejs.dev/config/
export default defineConfig({
  server: {
    host: "127.0.0.1",
    port: Number(process.env.WAILS_VITE_PORT) || 9245,
    strictPort: true,
  },
  plugins: [wails("./bindings"), keepDistPlaceholder, ...(process.env.VITE_MOCK === "1" ? [serveMockReport] : []), injectCsp],
});
