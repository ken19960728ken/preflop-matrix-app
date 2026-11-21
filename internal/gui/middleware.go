package gui

import "net/http"

const runtimePath = "/wails/runtime"

// guardRuntime 擋掉不是前端 Wails runtime 發出的 /wails/runtime 呼叫。
// v3.0.0 桌面版的 runtime 端點不檢查 method 與 Origin，且 body 為空時會從 query 讀參數，
// 所以 sandbox iframe（Origin: null）或導覽（GET）都能繞過報告頁的 CSP 呼叫綁定方法。
// 前端 runtime 只用 POST；其他 /wails/ 路徑（runtime.js、stream 的 GET poll）不受影響。
func guardRuntime(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == runtimePath {
			if r.Method != http.MethodPost {
				w.Header().Set("Allow", http.MethodPost)
				http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
				return
			}
			if r.Header.Get("Origin") == "null" {
				http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
