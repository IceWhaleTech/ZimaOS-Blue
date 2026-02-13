# ZimaOS-Blue CLI 使用教程

ZimaOS-Blue 提供了一个全面的命令行界面 (CLI)，用于管理和交互 NAS 原生 Agent 运行时。

## 目录

- [安装](#安装)
- [快速开始](#快速开始)
- [全局参数](#全局参数)
- [命令参考](#命令参考)
  - [服务管理](#服务管理)
  - [配置管理](#配置管理)
  - [模型管理](#模型管理)
  - [会话管理](#会话管理)
  - [定时任务](#定时任务)
  - [插件管理](#插件管理)
  - [技能管理](#技能管理)
  - [日志查看](#日志查看)
- [使用示例](#使用示例)
- [故障排除](#故障排除)

---

## 安装

### 从二进制文件安装

下载适合您平台的最新版本：

```bash
# Windows
curl -LO https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest/download/blue-windows-amd64.exe
mv blue-windows-amd64.exe blue.exe

# Linux
curl -LO https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest/download/blue-linux-amd64
chmod +x blue-linux-amd64
sudo mv blue-linux-amd64 /usr/local/bin/blue

# macOS
curl -LO https://github.com/IceWhaleTech/ZimaOS-Blue/releases/latest/download/blue-darwin-amd64
chmod +x blue-darwin-amd64
sudo mv blue-darwin-amd64 /usr/local/bin/blue
```

### 从源码编译

```bash
git clone https://github.com/IceWhaleTech/ZimaOS-Blue.git
cd ZimaOS-Blue/server
go build -o blue ./cmd/blue/
```

---

## 快速开始

### 1. 检查系统健康状态

```bash
# 运行诊断检查
blue doctor

# 自动修复常见问题
blue doctor --fix
```

### 2. 启动服务

```bash
# 前台运行（用于测试）
blue gateway run

# 或安装为系统服务
blue gateway install
blue gateway start
```

### 3. 检查状态

```bash
# 快速状态检查
blue status

# 显示所有详细信息
blue status --all
```

### 4. 查看日志

```bash
# 显示最近的日志
blue logs

# 实时跟踪日志
blue logs -f
```

---

## 全局参数

这些参数可以与任何命令一起使用：

| 参数 | 说明 |
|------|------|
| `--config <路径>` | 指定配置文件路径 |
| `--dev` | 开发模式（使用端口 8081，隔离状态） |
| `--profile <名称>` | 使用命名配置文件进行状态隔离 |
| `--no-color` | 禁用 ANSI 颜色输出 |
| `--json` | 以 JSON 格式输出（机器可读） |
| `-v, --verbose` | 启用详细输出 |
| `-h, --help` | 显示任何命令的帮助 |

### 示例

```bash
# 使用开发模式
blue --dev status

# 使用自定义配置文件
blue --profile testing config list

# 获取 JSON 输出用于脚本
blue --json models list

# 禁用颜色以便管道处理
blue --no-color logs | grep error
```

---

## 命令参考

### 服务管理

#### `gateway` - 服务控制

管理 ZimaOS-Blue 服务。

```bash
# 前台运行服务
blue gateway run [--port 23456] [--bind 0.0.0.0]

# 检查服务状态
blue gateway status

# 启动服务（后台）
blue gateway start

# 停止服务
blue gateway stop

# 重启服务
blue gateway restart

# 安装为系统服务
blue gateway install

# 卸载系统服务
blue gateway uninstall
```

**选项：**
- `--port <端口>` - HTTP 服务器端口（默认：23456）
- `--bind <地址>` - 绑定地址（默认：0.0.0.0）
- `--verbose` - 启用详细日志

#### `status` - 服务状态

显示服务健康状态和最近活动。

```bash
# 基本状态
blue status

# 显示所有详情
blue status --all

# 深度健康检查
blue status --deep

# JSON 输出
blue status --json
```

#### `health` - 健康检查

快速检查运行中服务的健康状态。

```bash
# 基本健康检查
blue health

# 自定义超时时间
blue health --timeout 5s

# JSON 输出
blue health --json
```

#### `doctor` - 系统诊断

运行诊断检查并自动修复问题。

```bash
# 运行所有检查
blue doctor

# 自动修复问题
blue doctor --fix
```

**执行的检查：**
- 配置目录是否存在
- 配置文件是否有效
- 数据目录是否存在
- 日志目录是否存在
- 服务是否运行
- 端口是否可用
- 依赖是否已安装
- 文件权限是否正确

---

### 配置管理

#### `config` - 配置管理

管理 ZimaOS-Blue 配置。

```bash
# 列出所有配置
blue config list

# 获取特定值
blue config get server.port

# 设置值
blue config set server.port 23456

# 删除值
blue config unset server.debug
```

**常用配置键：**
- `server.port` - HTTP 服务器端口
- `server.bind` - 绑定地址
- `server.debug` - 调试模式
- `providers.default` - 默认 AI 提供商
- `models.default` - 默认模型

---

### 模型管理

#### `models` - 模型管理

管理 AI 模型和提供商。

```bash
# 列出可用模型
blue models list

# 列出特定提供商的模型
blue models list --provider openai

# 检查模型可用性
blue models list --check

# 显示模型状态
blue models status

# 设置默认模型
blue models set gpt-4

# 扫描可用模型
blue models scan
```

**选项：**
- `--provider <名称>` - 按提供商过滤
- `--check` - 验证模型可用性
- `--json` - JSON 输出

---

### 会话管理

#### `sessions` - 会话管理

管理对话会话。

```bash
# 列出所有会话
blue sessions list

# 仅列出活跃会话
blue sessions list --active

# 显示会话详情
blue sessions show <会话ID>

# 删除会话
blue sessions delete <会话ID>

# 清除所有会话
blue sessions clear
```

**选项：**
- `--active` - 仅显示活跃会话
- `--json` - JSON 输出

---

### 定时任务

#### `cron` - 定时任务管理

管理计划的定时任务。

```bash
# 列出所有定时任务
blue cron list

# 显示定时服务状态
blue cron status

# 添加新的定时任务
blue cron add --name "每日备份" --cron "0 2 * * *" --handler http --payload '{"url":"http://localhost/backup"}'

# 删除定时任务
blue cron rm <任务ID>

# 启用/禁用任务
blue cron enable <任务ID>
blue cron disable <任务ID>

# 立即运行任务
blue cron run <任务ID>

# 查看任务执行历史
blue cron runs <任务ID>
```

**添加选项：**
- `--name <名称>` - 任务名称（必需）
- `--cron <表达式>` - Cron 表达式（必需）
- `--handler <类型>` - 处理器类型：http, command
- `--payload <json>` - 任务载荷（JSON 格式）

**Cron 表达式格式：**
```
┌───────────── 分钟 (0 - 59)
│ ┌───────────── 小时 (0 - 23)
│ │ ┌───────────── 日期 (1 - 31)
│ │ │ ┌───────────── 月份 (1 - 12)
│ │ │ │ ┌───────────── 星期 (0 - 6) (周日 = 0)
│ │ │ │ │
* * * * *
```

**示例：**
- `0 * * * *` - 每小时
- `0 2 * * *` - 每天凌晨 2 点
- `0 0 * * 0` - 每周日
- `*/15 * * * *` - 每 15 分钟

---

### 插件管理

#### `plugins` - 插件管理

管理 ZimaOS-Blue 插件。

```bash
# 列出所有插件
blue plugins list

# 显示插件详情
blue plugins info <插件ID>

# 启用/禁用插件
blue plugins enable <插件ID>
blue plugins disable <插件ID>

# 运行插件诊断
blue plugins doctor
```

**选项：**
- `--json` - JSON 输出

---

### 技能管理

#### `skills` - 技能管理

管理 Agent 技能。

```bash
# 列出所有技能
blue skills list

# 仅列出可用技能
blue skills list --eligible

# 显示技能详情
blue skills info <技能ID>

# 检查技能可用性
blue skills check
```

**选项：**
- `--eligible` - 仅显示可用技能
- `--json` - JSON 输出

---

### 日志查看

#### `logs` - 查看服务日志

查看和跟踪服务日志。

```bash
# 显示最近的日志（最后 50 行）
blue logs

# 显示最后 N 行
blue logs -n 100

# 实时跟踪日志
blue logs -f

# 按日志级别过滤
blue logs --level error

# 组合选项
blue logs -f --level warn -n 200
```

**选项：**
- `-f, --follow` - 实时跟踪日志
- `-n, --lines <数量>` - 显示的行数（默认：50）
- `--level <级别>` - 按级别过滤：debug, info, warn, error
- `--json` - JSON 输出

---

## 使用示例

### 使用 JSON 输出进行脚本编写

```bash
# 获取模型列表并用 jq 处理
blue --json models list | jq '.models[].name'

# 检查服务是否健康
if blue --json health | jq -e '.healthy' > /dev/null; then
    echo "服务健康"
else
    echo "服务不健康"
fi

# 获取会话数量
blue --json sessions list | jq '.conversations | length'
```

### 开发工作流

```bash
# 以开发模式启动
blue --dev gateway run

# 在另一个终端检查状态
blue --dev status

# 查看开发日志
blue --dev logs -f
```

### 配置文件隔离

```bash
# 创建测试配置文件
blue --profile testing config set server.port 9090

# 使用测试配置文件运行
blue --profile testing gateway run

# 每个配置文件都有隔离的：
# - 配置：~/.zimaos-blue-testing/config.yaml
# - 数据：~/.zimaos-blue-testing/data/
# - 日志：~/.zimaos-blue-testing/logs/
```

### 自动健康监控

```bash
#!/bin/bash
# health-check.sh

while true; do
    if ! blue health --timeout 5s > /dev/null 2>&1; then
        echo "$(date): 服务不健康，正在重启..."
        blue gateway restart
    fi
    sleep 60
done
```

---

## 故障排除

### 服务无法启动

1. **检查端口是否被占用：**
   ```bash
   blue doctor
   # 查看 "Port available" 检查结果
   ```

2. **检查日志中的错误：**
   ```bash
   blue logs --level error
   ```

3. **尝试前台运行：**
   ```bash
   blue gateway run --verbose
   ```

### 配置问题

1. **验证配置文件：**
   ```bash
   blue config list
   ```

2. **重置为默认值：**
   ```bash
   blue config unset <有问题的键>
   ```

3. **运行 doctor 并修复：**
   ```bash
   blue doctor --fix
   ```

### 连接被拒绝

1. **检查服务是否运行：**
   ```bash
   blue gateway status
   ```

2. **验证端口配置：**
   ```bash
   blue config get server.port
   ```

3. **检查防火墙设置**（因平台而异）

### 权限错误

1. **运行 doctor 检查权限：**
   ```bash
   blue doctor
   ```

2. **自动修复权限：**
   ```bash
   blue doctor --fix
   ```

### 找不到日志文件

CLI 会在以下位置查找日志：
1. `~/.zimaos-blue/logs/blue.log`
2. `./logs/blue.log`
3. `./blue.log`

确保服务至少启动过一次以创建日志文件。

---

## 获取帮助

```bash
# 通用帮助
blue --help

# 特定命令帮助
blue gateway --help
blue config --help
blue models --help
```

更多信息请访问：
- GitHub: https://github.com/IceWhaleTech/ZimaOS-Blue
- 文档: https://docs.zimaos.com/blue
