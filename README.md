# Tavern Shelf

Tavern Shelf 是 Tavern Player 的本地角色卡资源库。把 SillyTavern 资源放进 Shelf 自己的默认 Inbox 后，Shelf 会等待文件写入稳定、解析内容、按内容哈希去重，并把原始文件安全收录到自己管理的 Library。也可以长期监视外部目录、只扫描某个目录一次，或者直接把文件拖到 Shelf 窗口；这三种方式都只复制收藏，源文件始终留在原处。

当前仓库已经包含本地资源收录的完整纵向路径：

```text
Inbox -> stable-file scanner -> PNG / JSON parser -> Managed Library
      -> SQLite metadata -> categorized Library UI -> detail / source export
```

Windows 桌面入口额外提供系统托盘、关闭窗口后后台运行、单实例、静默启动参数、登录后自动启动开关和正常退出。相同的 Core 也通过独立的 headless 入口运行，方便未来部署到 Linux、NAS 或容器。

角色卡、独立世界书和预设都可以通过详情页的二维码，在同一局域网内创建一个十分钟有效的单资源传输会话。扫码端对接说明见 [Transfer Protocol v1](docs/transfer-protocol-v1.md)。

## 本地运行

需要 Go 1.26 或更新版本。修改或重新构建前端时还需要 Node.js 22.12 或更新版本与 npm；最终生成的 Tavern Shelf 程序会嵌入前端产物，普通用户不需要安装 Node.js。

Headless 模式：

```powershell
go run ./cmd/tavern-shelf
```

然后打开 <http://127.0.0.1:8787>。默认数据目录位于当前用户配置目录下，也可以显式指定：

```powershell
go run ./cmd/tavern-shelf -data-dir .\data -listen 127.0.0.1:8787
```

Windows 桌面开发模式：

```powershell
.\scripts\run-windows-dev.ps1
```

该脚本始终构建并运行 `build/dev/TavernShelf-dev.exe`，让 Windows 防火墙能够稳定识别同一个调试程序。使用 `go run ./cmd/tavern-shelf-desktop` 会在临时目录生成不同的 EXE，测试局域网二维码传输时可能反复出现防火墙确认。首次分享时只允许“专用网络”；调试程序仍在系统托盘运行时，应先从托盘退出再重新构建。

前端源码位于 `frontend/`，使用 Vue 3、Vite、TypeScript、Tailwind CSS、Reka UI primitives 和 Lucide。开发前端时可分别启动 Go headless 服务和 Vite：

```powershell
go run ./cmd/tavern-shelf
cd frontend
npm install
npm run dev
```

桌面入口使用 Wails v3 提供 WebView2 窗口、系统托盘和单实例能力。最终用户构建不带控制台窗口：

```powershell
.\scripts\build-windows.ps1
```

产物位于 `build/bin/TavernShelf.exe`。传入 `-Installer` 时，如果本机安装了 Inno Setup 6，还会生成当前用户安装包和开始菜单快捷方式。

## 数据目录

Shelf 只管理自己的根目录：

```text
Tavern Shelf/
├── Inbox/                 默认用户投递目录
├── Library/               正式收录的原始 Source
├── Trash/                 UI 删除后的可恢复内容，可从工具面板恢复
└── AppData/
    ├── shelf.db           SQLite 元数据
    ├── staging/           导入暂存目录
    └── duplicates/        完全相同的重复投递
```

原始卡是 source of truth。数据库字段和 UI 数据都可以从 Source 重建。导入时 Shelf 会先解析原文件，再把它复制到暂存目录并核对 SHA-256；只有托管副本和数据库记录都成功后，才会移除 Inbox 中的文件。解析失败或仍在写入的文件会留在 Inbox。

Shelf 提供三种明确的来源模式：

- 默认 Inbox：由 Shelf 接管，成功收录后原件进入 Managed Library；
- 长期监视目录：配置会在重启后恢复，只复制新内容，原文件保留；
- 扫描一次：只处理选择目录当时已有的顶层 PNG / JSON，不保存目录配置，原文件保留。

此外可以直接把一个或多个 PNG / JSON 拖到 Shelf 窗口完成收藏。拖拽上传先进入 Shelf 私有暂存区，校验并收录完成后清理暂存副本，不会修改拖拽来源。移除长期监视目录只会停止监视，不会删除目录或其中的文件。

## 当前支持

- SillyTavern JSON Character Card，包括常见 v1 和 v2 外层结构；
- Character Card V2 / V3 的标准结构化内容，并优先读取 PNG 中的 `ccv3` 数据；
- PNG `tEXt/chara`、`tEXt/ccv3` 元数据，以及未压缩或 zlib 压缩的 `iTXt` 元数据；
- 可重建的 Content Manifest：角色设定、开场与 alternate/group greetings、Character Book entry、Regex script 匹配信息、extension/asset 类型，以及 HTML、JavaScript 和已知交互扩展的存在性；
- 内容哈希去重，同名但内容不同的卡片不会互相覆盖；
- 可持久化配置多个 Inbox，并在运行中立即添加或移除扫描目录；
- 一次性目录扫描和窗口拖拽收藏，两者均采用复制语义并保留来源；
- 独立分发的 SillyTavern 世界书 JSON，按文件名命名并展示条目、关键词、启用状态与正文；
- SillyTavern 预设 JSON，识别 Chat Completion、Kobold、NovelAI、Text Generation、Context、Instruct、System Prompt 和 Reasoning 子类型；
- 角色、世界书和预设使用各自独立的 Library 分类；角色卡内嵌世界书只保留在角色详情中，不会重复创建独立世界书；
- 二维码传输只公布 RFC1918 私有 IPv4，优先物理局域网接口并降低 VPN、Hyper-V、WSL、Docker 等虚拟网卡优先级；
- 媒体库卡片浏览、搜索、详情、原始卡导出和移入 Shelf Trash；
- 完整 Library ZIP 备份与合并恢复；备份只包含受管原始资源和可移植清单，恢复时重新解析并校验 SHA-256；
- Shelf Trash 列表与一键恢复，恢复成功前不会清理 Trash 原文件；
- 工具面板显示当前无法收录的文件、解析错误、重试时间和一次性扫描问题；
- 轮询式稳定文件检测，坏文件使用冷却时间，避免高频错误循环。

Content Manifest 只对确定性字段做结构化投影，不执行卡片中的 HTML/JavaScript，也不尝试解释任意脚本语义。原始角色卡始终是 source of truth；旧 Library 会在升级后从 Managed Source 自动补建 Manifest。

V0 的边界和非目标见 [tavern-shelf-seed.md](tavern-shelf-seed.md)。

## 验证

```powershell
cd frontend
npm run build
cd ..
go test ./...
go build ./...
```

核心测试覆盖 JSON/PNG 解析、稳定文件检测、安全导入、重复内容和失败时保留原文件。

## License

Tavern Shelf 采用 [MIT License](LICENSE)。
