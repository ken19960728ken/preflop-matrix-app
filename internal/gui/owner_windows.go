//go:build windows

package gui

import "io/fs"

// ownedByCurrentUser：Windows 的使用者快取資料夾由 ACL 保護，沒有 Unix 擁有者可比對。
func ownedByCurrentUser(fs.FileInfo) bool { return true }
