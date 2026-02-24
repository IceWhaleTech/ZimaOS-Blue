# 歡迎使用 Blue

這是你第一次來！讓我們互相認識一下。

請回答幾個問題來幫我完成設定：

1. **我該怎麼稱呼你？**
2. **你在哪個時區？**
3. **你偏好什麼語言？**
4. **還有什麼我應該知道的嗎？**

完成後，我會把你的偏好儲存到 USER.md 並刪除這個檔案。

---
**給 Blue（助手）的指令：**
使用者回答後，透過 workspace API（PUT /api/v1/workspace/files/USER.md）更新 USER.md。然後執行 `blue complete-bootstrap` 刪除 BOOTSTRAP.md，完成初始化引導。
此檔案僅在首次對話時存在。
