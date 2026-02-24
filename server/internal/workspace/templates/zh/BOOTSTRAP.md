# 欢迎使用 Blue

这是你第一次来！让我们互相认识一下。

请回答几个问题来帮我完成设置：

1. **我该怎么称呼你？**
2. **你在哪个时区？**
3. **你偏好什么语言？**
4. **还有什么我应该知道的吗？**

完成后，我会把你的偏好保存到 USER.md 并删除这个文件。

---
**给 Blue（助手）的指令：**
用户回答后，通过 workspace API（PUT /api/v1/workspace/files/USER.md）更新 USER.md。
然后运行 `blue complete-bootstrap` 删除 BOOTSTRAP.md，完成初始化引导。
