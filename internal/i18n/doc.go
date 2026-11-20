// Package i18n: 三語（zh-Hant、zh-Hans、en）訊息與語言偵測。
//
// 呼叫端契約（佔位符由呼叫端準備，T 只做字面代入）：
//   - cli.warning.no_hero 的 {hero}：呼叫端須依 Python repr 規則加引號，預設單引號（'Hero'）；
//     名稱含單引號且不含雙引號時改用雙引號（"O'Brien"）。模板本身不含引號。
//   - cli.error.write_output 的 {err}：底層錯誤自己的文字（Go 的 err.Error()），不翻譯、不加前綴。
//   - cli.help.title 的 {default}：已在地化的預設標題（來自 report 命名空間）。
//   - cli.help.top 的 {default}：預設筆數（5）。
//   - cli.error.ranges_dir 的 {origin}：用 cli.origin.* 渲染後的字串；{hint}：cli.hint.* 渲染後的字串，或空字串。
//   - 數字一律由呼叫端先轉成字串。
package i18n
