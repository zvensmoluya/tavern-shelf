# Tavern Shelf — Project Seed

## 1. 项目定位

Tavern Shelf 是 Tavern Player 的本地角色卡资源库。

它主要运行在 Windows PC，也应保留未来运行在 NAS / Linux Server 上的可能。

用户通常在 PC 上通过 Discord、论坛、浏览器下载等方式获取 SillyTavern 社区角色卡，而实际游玩设备可能是 Android Tavern Player。

Shelf 的职责是：

> 收集、保存、整理用户自己的角色卡，并为 Tavern Player 提供一个稳定的内容来源。

它类似一个角色卡的私人唱片库。

不是公共社区，不是角色卡商店，也不是新的角色卡创作平台。

---

# 2. V0 核心用户路径

V0 首先只需要把下面这条路径做完整：

```text
用户获得一张角色卡
        ↓
放入 Tavern Shelf 的指定收件目录
        ↓
Shelf 自动发现
        ↓
识别并解析角色卡
        ↓
正式收录
        ↓
原始文件移动到 Shelf 自己管理的资源库
        ↓
角色出现在 Shelf UI
```

用户不应该继续手动管理已经收录的角色卡文件。

Shelf 应该像音乐库一样：

> 内容进入以后，由 Shelf 管理。

---

# 3. 技术限制

以下约束属于当前项目边界，实现不得轻易违反。

## 3.1 Go 作为 Core

核心业务逻辑使用 Go。

包括但不限于：

- 文件处理；
- Library；
- Scanner；
- Character Card parsing；
- persistence；
- future network transfer。

不要把核心业务绑定在 Web UI 或 Windows UI 中。

---

## 3.2 Windows 必须是正常桌面应用体验

Windows 用户不能通过：

```text
go run
cmd
PowerShell
常驻 terminal
```

来使用 Tavern Shelf。

Windows 版本需要表现为普通桌面应用：

- 有应用图标；
- 可从开始菜单启动；
- 支持系统托盘；
- 关闭窗口后可以继续后台运行；
- 可以从托盘重新打开；
- 可以正常退出；
- 支持用户登录后自动启动；
- 自动启动时应允许静默进入后台；
- 应避免多个 Shelf 实例同时运行。

具体 Desktop Framework 不强制指定。

当前可以优先研究适合 Go 的轻量 Desktop Host。

---

## 3.3 必须保留 Headless / NAS 可能性

Shelf Core 不能依赖 Windows GUI 才能运行。

长期应能够支持类似：

```text
Windows Desktop
Linux
NAS
Docker
Headless Server
```

的形态。

不要求 V0 完成所有这些部署方式，但架构不能把它们从根本上堵死。

---

## 3.4 UI 技术不限

可以使用：

- Vue；
- Vanilla Web；
- 其他适合的 Web UI；
- 或其他合理方案。

不要求使用某个特定前端框架。

选择标准是：

- 开发简单；
- UI 足够舒服；
- 容易维护；
- 能与 Go Core 清晰分离；
- 不给 NAS / Headless 带来明显阻碍。

不要为了技术栈本身增加复杂度。

---

## 3.5 Managed Library

成功收录的内容必须进入 Shelf 管理的 Library。

不要长期只保存用户原始路径，例如：

```text
Downloads/foo.png
Desktop/card.png
Discord/downloads/xxx.json
```

Shelf 应拥有自己的数据目录。

概念上至少存在：

```text
Inbox
Library
Application Data
```

具体目录组织方式由实现决定。

---

## 3.6 Source 必须安全保留

角色卡原始内容是 source of truth。

未来可以存在：

- metadata；
- thumbnail；
- cache；
- AI adaptation；
- compiled artifact；
- Tavern Player-specific derived data。

但这些都不能破坏原始 Source。

任何 derived artifact 都应该可以删除后重新生成。

---

## 3.7 导入不得导致文件丢失

如果角色卡：

- 无法解析；
- 格式未知；
- 文件仍在复制；
- 导入过程中出现异常；

原始文件不能因此丢失。

只有确认收录完成后才能正式将内容从 Inbox 转入 Managed Library。

---

# 4. Inbox

Shelf 应有一个明确的 Inbox / 收件目录。

用户可以把从：

- Discord；
- 浏览器；
- 网盘；
- 社区；

获得的角色卡放进去。

Shelf 自动检测新增内容。

Inbox 的语义应该明确：

> 放入这里的支持内容会被 Tavern Shelf 自动收录，并进入 Shelf 管理的 Library。

不要默认扫描整个 Downloads 或 Desktop 后擅自搬走文件。

---

# 5. 自动扫描

V0 的主要主动功能就是 Scanner。

最低要求：

- 应用启动后扫描 Inbox；
- 应用运行时能够发现后来加入的内容；
- 不读取明显仍在写入中的文件；
- 同一个文件不能被无限重复导入；
- 坏文件不能形成高频错误循环；
- 成功导入后自动进入 Library。

具体使用：

- filesystem watcher；
- polling；
- debounce；
- queue；

由实现决定。

---

# 6. V0 角色卡支持

优先支持 SillyTavern Character Card。

初期至少考虑：

- PNG Character Card；
- JSON Character Card。

不要求 V0 完整实现 SillyTavern Runtime。

Shelf 当前只需要获得足够用于：

- 识别；
- 收藏；
- 展示；
- 后续传输；

的信息。

例如可能包括：

- Name；
- Avatar；
- Creator；
- Character Card spec/version；
- Tags；
- 是否包含 Worldbook；
- 是否包含 Regex；
- 是否存在 extension / interactive content。

具体解析结构由实现根据真实格式决定。

---

# 7. Library UI

UI 应该更像：

> 唱片库 / 媒体库

而不是：

> 数据库管理后台。

主要视觉元素应该是：

- Avatar；
- Character Name；
- Creator；
- Library browsing。

V0 至少需要：

## Library 首页

能够浏览已经收录的角色。

优先采用封面 / 卡片形式。

例如：

```text
[Avatar] [Avatar] [Avatar]

角色 A   角色 B   角色 C
```

## Character Detail

点击角色能够看到基本信息。

例如：

- Avatar；
- Name；
- Creator；
- Card format；
- 基础 feature 信息；
- 收录时间；
- 必要的 source 信息。

不要为了 V0 建设复杂管理后台。

---

# 8. Windows 后台体验

Shelf 预计是一个轻量常驻程序。

用户体验应该类似：

```text
Windows 登录
↓
Shelf 静默启动
↓
后台监视 Inbox
↓
发现新角色卡
↓
自动收录
```

平时不需要一直显示主窗口。

应用窗口关闭后，应能够继续运行。

用户明确选择“退出”时才结束后台功能。

后台行为应尽量安静，不频繁打扰用户。

---

# 9. Library 删除

Shelf 管理的角色允许被删除。

因为删除意味着删除 Shelf 管理的原始内容，所以必须有明确确认。

具体：

- 是否使用回收站；
- 是否软删除；
- derived artifact 如何清理；

暂不规定。

V0 至少不能误删其他非 Shelf 管理文件。

---

# 10. 重复内容

完全相同的角色卡不应该因为多次放入 Inbox 而无限产生副本。

应该有可靠的方法识别完全相同的 source。

但：

> 同一个角色名 ≠ 同一张角色卡。

新版角色卡不能简单因为名称一致而被覆盖。

具体版本管理暂不属于 V0。

---

# 11. Tavern Player 关系

当前 V0 不要求完整完成 Tavern Player Android 集成。

但 Tavern Shelf 的 Core 应考虑未来能够：

```text
Shelf
↓
LAN / QR / Link
↓
Tavern Player
```

让手机获得 Shelf 中的角色卡。

因此不要把所有访问逻辑只能封闭在 Desktop UI 内。

具体 transfer protocol 后续设计。

---

# 12. AI / Character Adaptation

V0 不实现。

未来可能存在：

```text
Original ST Card
        ↓
Character Adaptation
        ↓
Tavern Player-ready artifact
```

Tavern Shelf 很适合作为这类处理的宿主。

但现在不要为了未来 AI Compiler 提前建设复杂系统。

只需保证：

> 原始 Source 与未来 Derived Artifact 可以自然共存。

即使 AI adaptation 永远不实现，Tavern Shelf 也必须仍然是一个完整有用的项目。

---

# 13. 明确不做

V0 不做：

- Public Hub；
- 公共角色卡搜索；
- Discord API 抓卡；
- 用户账户；
- 云平台；
- 评论；
- 点赞；
- 创作者系统；
- Tavern Player 专属角色卡格式；
- Character Editor；
- Worldbook Editor；
- Regex Editor；
- Tavern Helper runtime；
- JavaScript compatibility runtime；
- AI Compiler；
- 复杂标签管理；
- 复杂版本控制；
- 数据分析 Dashboard。

不要因为实现过程中“顺手”而扩大范围。

---

# 14. 可以自由决定的事情

以下内容不属于产品约束，由施工者根据实际工程自行决定：

- Desktop Framework；
- Web Framework；
- UI library；
- SQLite driver；
- ORM / SQL strategy；
- directory internal layout；
- watcher implementation；
- import queue implementation；
- internal package layout；
- API design；
- state management；
- frontend routing；
- CSS solution；
- build strategy。

优先选择简单、稳定、容易维护的实现。

不要为了所谓“未来扩展性”提前建设目前没有需求的抽象。

---

# 15. V0 验收

第一阶段只需要稳定完成下面这个闭环：

```text
安装 / 启动 Tavern Shelf
        ↓
拥有一个 Inbox
        ↓
放入一张有效角色卡
        ↓
Shelf 自动发现
        ↓
成功解析
        ↓
自动收录到 Managed Library
        ↓
角色出现在 Library UI
        ↓
可以打开角色详情
        ↓
关闭主窗口
        ↓
Shelf 继续在后台工作
        ↓
重新打开窗口后内容仍然存在
```

Windows 额外要求：

```text
用户登录后能够自动启动
+
不会出现终端窗口
+
重复启动不会产生多个后台实例
+
用户可以通过 Tray 正常打开和退出
```

完成这一闭环，即视为 Tavern Shelf V0 的第一阶段成功。

---

# 16. 当前工作原则

不要先设计完整 Tavern Shelf。

先让：

> **Drop card → Shelf appears**

这件事情稳定工作。

实现过程可以根据真实工程自行调整。

如果实现过程中发现当前需求之间存在矛盾，应优先保持：

1. 用户文件安全；
2. Windows 使用简单；
3. Core 可脱离 Windows UI；
4. Managed Library 清晰；
5. 项目范围足够小。

其他设计都可以修改。
