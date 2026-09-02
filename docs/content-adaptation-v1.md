# Content adaptation v1

Shelf 可把依赖酒馆助手、HTML 或 JavaScript 的社区角色卡转换为 Tavern Player 能解释的声明式原生 UI。此流程遵守两条不可跨越的边界：原始角色卡始终是事实来源；任何卡内 JavaScript 都不会在 Shelf 或 Player 中执行。

## 数据流

1. Shelf 从受管理原件的 PNG 元数据或 JSON 中提取 `ProgramView`。
2. `ProgramView` 只包含活跃标记、脚本结构、依赖句柄和行为线索。角色描述、开场白、世界书正文等叙事内容只留下长度与 SHA-256。
3. 提取器移除凭据、本机用户路径、内联 data URI，并把远程 URL 替换为去凭据和查询参数的依赖句柄。
4. 模型只接收这个脱敏视图，并提议 `AdaptationArtifact`。
5. Shelf 使用与 Player 相同的 schema、类型、引用、尺寸和 capability 白名单做确定性校验。模型输出不是信任边界。
6. 通过校验的产物作为可重建派生文件保存；原件字节不会变化。

当前 Player v1 只接受：

- 原生 `SECTION`、`TEXT`、`STATUS`、`FORM` 组件；
- 文本、多行文本、数字、单选、多选和开关字段；
- 写入聊天草稿，以及有类型的会话状态 set/increment/toggle；
- `MESSAGE_REPLACEMENT`、`MESSAGE_ATTACHMENT` 与 `CONVERSATION_HEADER` 三种放置方式。

网络访问、任意脚本、WebView、DOM、文件 URI 和未声明变量不会进入产物。无法映射的行为必须报告为 `PARTIAL`。

## 派生文件

每张角色卡的受管理目录可以包含：

```text
Library/<prefix>/<source-hash>/
  source.png
  derived/
    program-view-v1.json
    adaptation-v1.json
```

SQLite 的 `character_adaptations` 表记录原件 SHA-256、两个派生文件的 SHA-256、编译器信息和状态。删除角色时目录整体进入 Shelf Trash；派生文件可以随时从原件重建。

## 本地编译命令

编译器使用 OpenAI Responses 兼容接口。凭据从环境变量读取，不应写入命令历史或仓库：

```powershell
go run ./cmd/tavern-adapt -data-dir "D:\Tavern Shelf Data" -character <source-sha256>
```

默认读取 `TAVERN_TEST_BASE_URL`、`TAVERN_TEST_API_KEY` 和 `TAVERN_TEST_MODEL`。加上 `-program-view-only` 只建立脱敏视图，不发送模型请求。
