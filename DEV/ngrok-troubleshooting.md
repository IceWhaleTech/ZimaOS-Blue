# Ngrok 远程访问故障排除指南

## 常见问题

### 1. 杀毒软件拦截 Ngrok

**症状:**
- 点击"启动远程访问"后没有反应
- 状态一直显示"正在建立安全隧道"
- 杀毒软件弹出警告或自动删除 ngrok.exe

**原因:**
许多杀毒软件会将 ngrok 标记为潜在不需要的程序(PUP)或恶意软件,因为它可以创建绕过网络限制的隧道。这是误报。

**解决方案:**

#### Windows Defender
1. 打开 Windows 安全中心
2. 点击"病毒和威胁防护"
3. 点击"管理设置"
4. 向下滚动到"排除项"
5. 点击"添加或删除排除项"
6. 添加以下路径:
   - `%USERPROFILE%\.zimaos-echo\data\ngrok\ngrok.exe`
   - 或者添加整个文件夹: `%USERPROFILE%\.zimaos-echo\data\ngrok\`

#### 其他杀毒软件

**Avast/AVG:**
1. 打开 Avast/AVG
2. 菜单 → 设置 → 常规 → 例外
3. 添加文件路径: `%USERPROFILE%\.zimaos-echo\data\ngrok\ngrok.exe`

**Kaspersky:**
1. 打开 Kaspersky
2. 设置 → 其他 → 威胁和排除项 → 排除项
3. 添加 → 浏览 → 选择 ngrok.exe

**Norton:**
1. 打开 Norton
2. 设置 → 防病毒 → 扫描和风险 → 排除项/低风险
3. 配置 → 添加文件夹或文件

**McAfee:**
1. 打开 McAfee
2. 病毒和间谍软件防护 → 实时扫描
3. 排除的文件 → 添加文件
4. 浏览到 ngrok.exe

**360 安全卫士:**
1. 打开 360 安全卫士
2. 木马查杀 → 信任区
3. 添加信任文件 → 选择 ngrok.exe

**腾讯电脑管家:**
1. 打开腾讯电脑管家
2. 病毒查杀 → 信任区
3. 添加文件 → 选择 ngrok.exe

### 2. Windows 防火墙拦截

**症状:**
- Ngrok 启动但无法建立连接
- 防火墙弹出警告

**解决方案:**

#### 自动添加防火墙规则(推荐)
ZimaOS-Echo 使用 Windows COM API 自动添加防火墙规则:
- **优势**: 使用原生 Windows API,更可靠
- **要求**: 需要管理员权限
- **自动执行**: 首次启动 ngrok 时自动尝试添加

如果自动添加失败(通常是因为没有管理员权限),请使用以下方法之一:

#### 方法 1: 以管理员身份运行 ZimaOS-Echo
1. 右键点击 ZimaOS-Echo 快捷方式或可执行文件
2. 选择"以管理员身份运行"
3. 启动远程访问功能
4. 防火墙规则将自动添加

#### 方法 2: 手动通过 PowerShell 添加(推荐)
以管理员身份打开 PowerShell,运行:
```powershell
$ngrokPath = "$env:USERPROFILE\.zimaos-echo\data\ngrok\ngrok.exe"
New-NetFirewallRule -DisplayName "ZimaOS-Echo-Ngrok" `
    -Direction Inbound `
    -Action Allow `
    -Program $ngrokPath `
    -Enabled True `
    -Profile Any `
    -Description "Allow ZimaOS-Echo to use ngrok for remote access"
```

#### 方法 3: 通过 Windows 防火墙界面
1. 打开 Windows 安全中心
2. 点击"防火墙和网络保护"
3. 点击"允许应用通过防火墙"
4. 点击"更改设置"(需要管理员权限)
5. 点击"允许其他应用"
6. 浏览到: `%USERPROFILE%\.zimaos-echo\data\ngrok\ngrok.exe`
7. 添加并确保勾选"专用"和"公用"

#### 验证防火墙规则
检查规则是否已添加:
```powershell
Get-NetFirewallRule -DisplayName "ZimaOS-Echo-Ngrok"
```

或者使用诊断 API:
```bash
curl http://localhost:8080/api/v1/remote-access/diagnostics
```
查看 `firewall_exception` 字段。

### 3. Linux 防火墙配置

**症状:**
- Ngrok 启动但无法建立连接
- 防火墙阻止入站连接

**当前状态:**
目前 ZimaOS-Echo 尚未实现 Linux 防火墙的自动配置。

**未来计划:**
将集成 [ngrok/firewall_toolkit](https://github.com/ngrok/firewall_toolkit) 来自动配置 Linux 防火墙:
- 支持 iptables
- 支持 ufw (Ubuntu/Debian)
- 支持 firewalld (RHEL/CentOS/Fedora)
- 支持 nftables

**临时解决方案:**

#### 使用 ufw (Ubuntu/Debian)
```bash
# 允许 Echo 监听的端口
sudo ufw allow 8080/tcp
sudo ufw reload
```

#### 使用 firewalld (RHEL/CentOS/Fedora)
```bash
# 允许 Echo 监听的端口
sudo firewall-cmd --permanent --add-port=8080/tcp
sudo firewall-cmd --reload
```

#### 使用 iptables
```bash
# 允许 Echo 监听的端口
sudo iptables -A INPUT -p tcp --dport 8080 -j ACCEPT
sudo iptables-save > /etc/iptables/rules.v4
```

### 4. 企业网络限制

**症状:**
- 在家里可以正常使用,但在公司网络无法使用
- 连接超时

**原因:**
企业网络可能阻止了 ngrok 使用的端口或域名。

**解决方案:**
- 联系 IT 部门,请求允许访问 `*.ngrok.io` 和 `*.ngrok.com`
- 或者使用自己的手机热点进行测试

### 5. 检查 Ngrok 是否正在运行

**Windows:**
1. 打开任务管理器 (Ctrl+Shift+Esc)
2. 查找 "ngrok.exe" 进程
3. 如果存在但没有网络活动,可能被防火墙阻止

**查看日志:**
1. 在 ZimaOS-Echo 界面中,打开"远程访问"页面
2. 查看日志选项卡(如果有)
3. 或者查看服务器日志: `%USERPROFILE%\.zimaos-echo\logs\`

### 6. 端口冲突

**症状:**
- 错误消息提示端口已被占用

**解决方案:**
1. 更改 ZimaOS-Echo 使用的端口
2. 或者找到占用端口的程序并关闭它

查找占用端口的程序:
```cmd
netstat -ano | findstr :8080
tasklist /FI "PID eq [PID]"
```

## 最佳实践

### 1. 使用 Ngrok 认证令牌
免费的 ngrok 账户有限制,建议注册并使用认证令牌:
1. 访问 https://ngrok.com/
2. 注册免费账户
3. 获取认证令牌
4. 在 ZimaOS-Echo 设置中配置令牌

### 2. 定期更新
确保 ZimaOS-Echo 和 ngrok 都是最新版本。

### 3. 安全建议
- 不要在公共网络上长时间运行远程访问
- 使用强密码保护 ZimaOS-Echo
- 定期检查访问日志
- 不需要时及时关闭远程访问

## 仍然无法解决?

1. **查看详细日志:**
   - 服务器日志: `%USERPROFILE%\.zimaos-echo\logs\echo.log`
   - Ngrok 日志: 在数据库中查看 `remote_access_logs` 表

2. **测试 Ngrok 独立运行:**
   ```cmd
   cd %USERPROFILE%\.zimaos-echo\data\ngrok
   ngrok.exe http 8080
   ```
   如果独立运行正常,说明是 ZimaOS-Echo 集成问题。

3. **提交问题报告:**
   访问 GitHub Issues 并提供:
   - 操作系统版本
   - 杀毒软件名称和版本
   - 错误日志
   - 是否有企业网络限制

## 技术细节

### Ngrok 工作原理
1. Ngrok 在本地启动一个客户端进程
2. 连接到 ngrok 的云服务器
3. 创建一个公网 URL,转发到本地端口
4. 所有流量通过加密隧道传输

### 为什么杀毒软件会拦截?
- Ngrok 可以绕过防火墙和 NAT
- 可能被恶意软件用于建立后门
- 杀毒软件采用启发式检测,可能误报

### 安全性
- Ngrok 使用 TLS 加密
- 所有流量都经过 ngrok 服务器中转
- 免费版本的 URL 是随机生成的
- 付费版本可以使用自定义域名
