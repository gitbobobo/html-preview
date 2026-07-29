# HTML Preview

单二进制 Go 服务：上传 HTML 或 ZIP 静态页，生成可分享的公开预览链接，并在管理端以响应式缩略图网格浏览。内置 SQLite 元数据、文件存储、回收站与生命周期、可选 Chrome 无头截图（桌面/移动双 WebP）。

## 常用命令（pnpm）

在项目根目录：

| 命令 | 说明 |
|---|---|
| `pnpm build` | `CGO_ENABLED=0` 编译为 `./html-preview` |
| `pnpm dev` | `go run` 开发启动 |
| `pnpm start` | 先 build 再运行二进制 |
| `pnpm test` | 跑 Go 测试 |
| `pnpm vet` | `go vet` |
| `pnpm check` | vet + test + 前端 JS 语法检查 |
| `pnpm clean` | 删除本地二进制 |

## 构建

无 CGO，可直接交叉编译（或用 `pnpm build`）：

```bash
CGO_ENABLED=0 go build -o html-preview .
```

产物为当前目录下的 `html-preview` 可执行文件（已列入 `.gitignore`）。

## 运行

```bash
pnpm start
# 或开发态：pnpm dev
# 或：./html-preview
```

首次启动会在数据目录创建所需结构；默认监听 **`0.0.0.0:7849`**（局域网设备可用本机 IP 访问，启动日志与「设置」页会打印/展示局域网地址）。

> 若无法从其他设备打开：确认防火墙放行 `7849`，且未将 `HOST` 设为 `127.0.0.1`。`pnpm start` / `pnpm dev` 会显式使用 `HOST=0.0.0.0`。

### 环境变量

| 变量 | 默认 | 说明 |
|---|---|---|
| `HOST` | `0.0.0.0` | 监听地址（默认全网卡，支持局域网访问；仅本机可设 `127.0.0.1`） |
| `PORT` | `7849` | 监听端口 |
| `HTML_PREVIEW_DATA` | `./data` | 数据根目录（数据库、内容、Chrome、本地配置） |
| `CHROME_PATH` | 空 | 浏览器可执行文件路径（优先于 `data/config.json` 与 PATH 探测） |
| `LIFECYCLE_INTERVAL` | `1m` | 过期扫描与回收站清理间隔（Go `time.ParseDuration` 格式，无效则回退 1 分钟） |

示例：

```bash
export HTML_PREVIEW_DATA=/var/lib/html-preview
export PORT=7849
./html-preview
```

## 首次使用

1. **设密**：浏览器打开 `http://localhost:7849/setup`，创建管理密码（仅首次；之后走 `/login`）。
2. **API Key**：登录后进入「设置」，创建 API Key；完整密钥**只显示一次**，请妥善保存。Agent 可将 `base_url` 与 `api_key` 写入 `~/.config/html-preview/config.yaml`（详见 `skills/html-preview/SKILL.md`）。
3. **浏览器 / 截图**：设置页可查看 Chrome 探测状态；若无可用浏览器，可点击「一键安装」拉取 Chrome for Testing 到数据目录（无需 root）。安装完成后自动为待截图项生成约 640px 宽的桌面/移动 WebP 缩略图。

## 数据目录布局

`HTML_PREVIEW_DATA`（默认 `./data`）运行时结构：

```
data/
  html-preview.db          # SQLite：用户、会话、API Key、预览项元数据
  config.json              # 本地配置，如 chrome_path、chrome_source
  chrome/                  # 一键安装的 Chrome for Testing（可选）
  items/{id}/
    index.html             # HTML 上传或 ZIP 根 index.html
    ...                    # ZIP 解压的相对资源
    preview_desktop.webp   # 桌面缩略图（截图成功后）
    preview_mobile.webp    # 移动缩略图
```

彻底删除会移除数据库记录并 `RemoveAll(items/{id}/)`。

`data/` 目录与二进制已加入 `.gitignore`，勿提交到版本库。

## 公开预览与管理页

| 路径 | 说明 |
|---|---|
| `/c/{id}/` | 公开预览（无鉴权；仅 active 且未过期） |
| `/` | 管理首页：搜索、上传、网格 |
| `/trash` | 回收站 |
| `/settings` | 密码、API Key、浏览器 |
| `/setup` / `/login` | 首次设密 / 登录 |

## Agent 集成

程序化调用说明见 [`skills/html-preview/SKILL.md`](skills/html-preview/SKILL.md)：认证配置、Items API、上传限制、错误码与 curl 示例。
