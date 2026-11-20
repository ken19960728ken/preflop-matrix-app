import { writeFileSync } from "node:fs";
import { defineConfig } from "vite";
import wails from "@wailsio/runtime/plugins/vite";

// vite 建置會清空 dist；補回 .gitkeep，讓乾淨 clone 的 go:embed all:frontend/dist 有檔可嵌。
const keepDistPlaceholder = {
  name: "keep-dist-placeholder",
  apply: "build" as const,
  closeBundle() {
    writeFileSync(new URL("./dist/.gitkeep", import.meta.url), "");
  },
};

// https://vitejs.dev/config/
export default defineConfig({
  server: {
    host: "127.0.0.1",
    port: Number(process.env.WAILS_VITE_PORT) || 9245,
    strictPort: true,
  },
  plugins: [wails("./bindings"), keepDistPlaceholder],
});
