// Package gui: Wails 桌面視窗（唯一 import wails 的套件）。
//
// AppService 的 http.Handler 掛在路由 /report 下；Wails 的資產伺服器先比對服務路由再回到前端資產，
// 所以前端資產（frontend/dist）的路徑不得以 /report 開頭，否則會被報告 handler 攔走。
package gui
