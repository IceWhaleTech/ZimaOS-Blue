# ZimaOS Blue v0.10.39

發布日期：2026-04-09

本次更新統一了 Research 工作流程，新增 Knowledge 與 Evolution 工作區，並進一步強化了對話恢復能力，以及文件擷取與執行可靠性。

## 亮點

- 為 research、knowledge、evolution 任務帶來更一致的工作流程
- 返回進行中或等待中的對話時，恢復體驗更好
- 文件擷取與受控執行的可靠性更高

## 新增

- 新增統一的 `Research` 入口，同時保留不同模式下的專屬輸出
- 新增 `Knowledge` 工作區，用於管理編譯頁面、lint 狀態與 ask-and-archive 任務
- 新增 `Evolution` 主控台與本地 `blue audit` 工具，用於審查與診斷

## 改進

- 改進對話 bootstrap，使重新開啟的聊天可恢復更多活動狀態與待處理批准
- 改進 Harness 資料集工作流程，讓評估執行更具可重複性
- 改進上下文壓縮、failover 處理與在地化一致性

## 修復

- 修復供應商目錄驗證中的重新整理迴圈問題
- 透過 artifact recovery 處理修復重複讀取迴圈情境
- 修復 PDF 文字擷取過程中的異常字元修復問題

## 安全

- 透過 `blue exec` 與分層 sandbox 路由強化命令執行安全性

## 說明

如果你遇到任何問題，歡迎加入 Zima Discord 社群，獲得超過 43,000 名成員的支援：

https://zimaboard.com/discord
