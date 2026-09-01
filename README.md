# Tavern Shelf

Tavern Shelf 是 Tavern Player 的本地角色卡资源库。把 SillyTavern 角色卡放进明确的 Inbox 后，Shelf 会等待文件写入稳定、解析卡片、按内容哈希去重，并把原始文件安全收录到自己管理的 Library。

当前仓库已经包含 V0 的第一条完整纵向路径：

```text
Inbox -> stable-file scanner -> PNG / JSON parser -> Managed Library
      -> SQLite metadata -> Library UI -> character detail / source export
```

Windows 桌面入口额外提供系统托盘、关闭窗口后后台运行、单实例、静默启动参数、登录后自动启动开关和正常退出。相同的 Core 也通过独立的 headless 入口运行，方便未来部署到 Linux、NAS 或容器。

## 本地运行

需要 Go 1.26 或更新版本。修改或重新构建前端时还需要 Node.js 22 与 npm；最终生成的 Tavern Shelf 程序会嵌入前端产物，普通用户不需要安装 Node.js。

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
go run ./cmd/tavern-shelf-desktop
```

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
├── Inbox/                 用户投递目录
├── Library/               正式收录的原始 Source
├── Trash/                 UI 删除后的可恢复内容
└── AppData/
    ├── shelf.db           SQLite 元数据
    ├── staging/           导入暂存目录
    └── duplicates/        完全相同的重复投递
```

原始卡是 source of truth。数据库字段和 UI 数据都可以从 Source 重建。导入时 Shelf 会先解析原文件，再把它复制到暂存目录并核对 SHA-256；只有托管副本和数据库记录都成功后，才会移除 Inbox 中的文件。解析失败或仍在写入的文件会留在 Inbox。

## 当前支持

- SillyTavern JSON Character Card，包括常见 v1 和 v2 外层结构；
- Character Card V2 / V3 的标准结构化内容，并优先读取 PNG 中的 `ccv3` 数据；
- PNG `tEXt/chara`、`tEXt/ccv3` 元数据，以及未压缩或 zlib 压缩的 `iTXt` 元数据；
- 可重建的 Content Manifest：角色设定、开场与 alternate/group greetings、Character Book entry、Regex script 匹配信息、extension/asset 类型，以及 HTML、JavaScript 和已知交互扩展的存在性；
- 内容哈希去重，同名但内容不同的卡片不会互相覆盖；
- 媒体库卡片浏览、搜索、详情、原始卡导出和移入 Shelf Trash；
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
