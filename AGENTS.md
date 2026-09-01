# Tavern Shelf repository guidance

## Product direction

- 默认使用中文沟通；代码、标识符和提交标题使用英文。
- 开始产品或架构改动前先阅读 `tavern-shelf-seed.md`。它是当前 V0 的产品边界；用户在当前任务中的明确要求优先级更高。
- V0 的首要闭环是：`Drop card -> Shelf appears`。优先保证用户文件安全、Windows 使用简单、Core 可脱离 GUI、Managed Library 清晰和实现范围克制。
- Tavern Shelf 是用户私有的角色卡媒体库，不是公共社区、商店、编辑器或 SillyTavern runtime。不要顺手扩展种子文件“明确不做”的功能。
- 项目尚未发布，允许清晰的非兼容重构。删除被替代的路径，并同步调整测试与文档，不保留无实际用户的兼容层。

## Architecture boundaries

- 核心业务使用 Go，放在 `internal/` 中；不得依赖 Windows UI、浏览器生命周期或桌面框架。
- `cmd/tavern-shelf` 是跨平台 headless 入口；`cmd/tavern-shelf-desktop` 是 Windows 桌面壳。HTTP API/UI 应调用同一套 Core，而不是复制业务逻辑。
- 数据目录必须明确区分 Inbox、Managed Library 和应用数据。只允许删除或移动 Shelf 自己管理范围内且经过校验的文件。
- 原始角色卡是 source of truth。解析结果、缩略图和其他 derived artifact 必须可重建，不能改写原始 source。
- Content Manifest 只投影规范中可确定性解析的内容。不得执行卡片内 HTML/JavaScript、猜测任意脚本语义或引入 AI 补全；Manifest schema 变化时应从 Managed Source 安全重建。
- 导入采用先解析、校验和复制到暂存文件，再原子提交元数据，最后处理 Inbox 原文件的顺序。失败时保留原文件；禁止以“清理”为由吞掉未知或损坏内容。
- 内容去重以 source 内容哈希为准，不以角色名称为准。不同内容的同名角色可以并存。
- 优先使用标准库和少量成熟依赖。不要为未确认的未来传输、AI adaptation 或多用户需求预建抽象。

## Go conventions

- 目标版本以 `go.mod` 为准。提交前运行 `gofmt`；包名简短、全小写，错误应包含操作上下文。
- 文件系统操作使用显式路径和最小权限；创建目录使用合理权限。涉及用户文件的逻辑必须有失败路径测试。
- HTTP handler 只负责协议转换与输入校验；Scanner、Parser、Library 和 Store 保持可独立测试。
- Web UI 作为嵌入资源随 Go 程序发布，避免要求最终用户安装 Node.js 或保持终端运行。
- 前端源码位于 `frontend/`，明确使用 Vue 3、Vite、TypeScript 和 Tailwind CSS；生产构建输出到 `internal/webui/static/` 后由 Go embed 打包。
- 通用交互优先复用无样式 headless primitives 和 `components/ui` 中的 Shelf 组件。禁止重新引入 `innerHTML` 拼接页面或扩张页面级重复 CSS。
- 所有通用功能图标只使用 Lucide。品牌标识和系统托盘图标是独立产品资产，不能用 Lucide 图标直接代替。
- 平台专属代码使用 Go build tags 隔离；非 Windows 构建不得导入 Windows API。

## Verification

- 默认先运行与改动直接相关的测试，再运行 `go test ./...`。
- 改动 Go 依赖或构建边界后运行 `go mod tidy` 和 `go build ./...`。
- Scanner/importer 测试至少覆盖：有效 JSON、有效 PNG、重复内容、未稳定文件、解析失败保留原文件、成功后进入 Managed Library。
- UI 改动至少验证空库、卡片列表、详情、错误状态和窄窗口布局。能自动化时优先自动化；不能自动化时说明手工验证项。
- 前端改动至少运行 `npm run build`（包含 `vue-tsc`）；依赖锁文件必须与 `package.json` 同步。
- Windows 桌面行为应验证：无控制台窗口、单实例、关闭窗口后驻留、托盘打开/退出、开机自启开关。无法在当前环境执行的系统级检查必须明确记录。

## Repository hygiene

- 不提交密钥、凭据、本机绝对路径、用户 Library、SQLite 数据库、构建产物或缓存。
- 保留与当前任务无关的用户改动；不要使用破坏性 Git 命令处理它们。
- 代理可以根据工作阶段的完整性自行决定是否创建提交；涉及不完整实现、失败验证或与当前任务无关的用户改动时不要提交。提交信息采用 Conventional Commits：`type(scope): summary`，summary 使用简短英文祈使语气。
