# CSGClaw Electron Desktop 改造方案

> - 评估日期：2026-07-24
> - CSGClaw 代码基线：`f6559fed`（`upstream/main`）
> - 文档分支基线：`7fbd4be3`
> - Electron 参考源码：`reference-projects/electron`，提交 `907fd5d`
> - 范围：Electron Shell、Go sidecar、Web UI 适配、安全边界、桌面生命周期、打包、签名、发布与升级
> - 结论性质：跨前端、后端和发布链路的实施方案，不包含本次代码改造

关联文档：

- [后端架构评估](backend-architecture-assessment.zh.md)
- [前端架构评估](web/architecture-assessment.zh.md)
- [Mantine 采用与渐进迁移方案](web/mantine-adoption-plan.zh.md)

## 1. 总结

### 1.1 一句话结论

CSGClaw 应采用“**Electron 管桌面生命周期，Go 继续作为唯一业务后端并提供现有 Web UI**”的 sidecar 架构。

> 第一阶段不把 React 应用迁到 `file://`，不把 Go 能力重写成 Node IPC，也不建立第二套 Desktop 业务实现。

Electron 是新的 Desktop Adapter，负责：

- 单实例、窗口、菜单、托盘和系统通知。
- 启动、监控和优雅停止 CSGClaw Go sidecar。
- OAuth 外部浏览器、受控桌面 IPC 和桌面升级。
- 安装包、代码签名、公证和桌面发布。

Go sidecar 继续负责：

- Agent、IM、Task、Team、Runtime、Sandbox 和模型链路。
- 本地持久化与配置。
- HTTP JSON、SSE、WebSocket 和 OAuth Callback。
- 提供已经构建并嵌入的 `web/static-dist`。

React Renderer 继续负责：

- 现有产品 UI 和路由。
- 通过同源相对 URL 调用 Go API。
- 通过一个很窄的 `DesktopBridge` 使用必要的桌面能力。

### 1.2 核心决策

| 决策           | 处理方式                                                                 |
| -------------- | ------------------------------------------------------------------------ |
| 产品形态       | 保留 CLI/Web，新增 Electron Desktop，不替换现有入口                      |
| Electron 定位  | Desktop Adapter 和 Go sidecar Supervisor                                 |
| 业务后端       | 继续以 Go 为唯一真相，不在 Electron Main 复制业务逻辑                    |
| Renderer 来源  | 生产环境加载 Go sidecar 的 loopback HTTP 页面                            |
| Renderer 技术  | 复用现有 `web/app`，不建立第二套 Desktop React 应用                      |
| 进程通讯       | 业务数据继续使用 HTTP/SSE/WebSocket；IPC 只处理桌面能力                  |
| Go 启动方式    | 新增稳定的 desktop sidecar 启动契约，不复用人类可读 CLI 输出             |
| 端口           | `127.0.0.1:0` 动态端口，由 Go 监听后报告实际地址                         |
| 本机鉴权       | 每次启动生成随机 session token，由 Electron Session 注入请求头           |
| 退出语义       | 关窗口默认保留后台运行；显式 Quit 才优雅停止 Electron 自己启动的 sidecar |
| 升级所有权     | Desktop 整包由 Electron 升级；Go 内置升级在 Desktop 模式禁用             |
| 打包工具       | 优先 Electron Forge，不自研 Electron 打包器                              |
| Electron 版本  | 实施时固定当前 Stable 精确版本，建立持续升级节奏                         |
| 第一条验证链路 | 先完成 macOS arm64 开发闭环，再扩 Windows、macOS x64 和 Linux            |

### 1.3 成功标准

改造完成后，应满足：

1. 同一份 `web/app` 同时运行在普通浏览器和 Electron 中。
2. Electron 不知道 Agent、Task、Room 等领域对象。
3. Go sidecar 崩溃、端口冲突、版本不匹配时，Electron 能给出明确恢复入口。
4. Renderer 即使发生 XSS，也不能直接访问 Node、文件系统、任意 IPC 或任意本地进程。
5. CLI/Web 的现有安装、服务和升级链路不被 Desktop 改造破坏。
6. Desktop 安装包中的 Electron、Go binary、`csgclaw-cli`、BoxLite 和 Sandbox CLI 作为一个版本整体发布。

## 2. 为什么推荐 Sidecar，而不是直接把 Web 塞进 Electron

### 2.1 CSGClaw 当前运行事实

当前 CSGClaw 已经形成完整的浏览器运行链路：

```text
csgclaw serve
  -> cli/serve 组装应用
  -> internal/server 启动 HTTP Server
  -> web.Handler 提供嵌入的 web/static-dist
  -> React 使用相对 URL 请求 api/v1
  -> SSE SharedWorker 连接 api/v1/events
```

关键代码依据：

| 能力             | 当前位置                                            | 对 Electron 的影响                               |
| ---------------- | --------------------------------------------------- | ------------------------------------------------ |
| Web UI Go Embed  | `web/ui.go`                                         | Go binary 已经能独立提供完整 Renderer            |
| UI Fallback      | `internal/server/ui.go`                             | Hash Router 和静态资源无需 Electron 重做         |
| 应用装配         | `cli/serve/serve.go`                                | 需要抽取稳定 Bootstrap，避免 Electron 复制装配   |
| HTTP Listener    | `internal/server/http.go`                           | 需要支持动态端口 Ready Contract                  |
| API 相对路径     | `web/app/src/api/client.ts`                         | 继续同源加载可避免 API Base URL 改造             |
| Hash Router      | `web/app/src/routes/AppRouter.tsx`                  | 从 loopback HTTP 加载时可直接复用                |
| SSE SharedWorker | `web/app/src/shared/realtime/*`                     | 同源 HTTP 最容易保持现有行为                     |
| OAuth            | `useAuthController.ts`、`useConnectorController.ts` | 需要 Desktop External Navigation Adapter         |
| Go 自升级        | `internal/upgrade/`                                 | Desktop 中必须让位于整包升级                     |
| Release Bundle   | `scripts/package-release.sh`                        | 可作为 Electron `extraResources` 的 sidecar 输入 |

当前 `Vite` 已配置：

- `base: "./"`。
- 开发代理 `/api` 和 `/healthz` 到 `127.0.0.1:18080`。
- 单独输出 `sse-shared-worker.js`。
- 生产构建输出到 `web/static-dist` 并由 Go Embed。

这意味着保留 Go HTTP Origin，可以最大程度复用现有前端。

### 2.2 Electron 官方仓库给出的约束

本次核对了本地 `reference-projects/electron` 中的以下官方材料：

- `docs/tutorial/process-model.md`
- `docs/tutorial/security.md`
- `docs/tutorial/context-isolation.md`
- `docs/tutorial/sandbox.md`
- `docs/tutorial/ipc.md`
- `docs/tutorial/application-distribution.md`
- `docs/tutorial/tutorial-5-packaging.md`
- `docs/tutorial/updates.md`
- `docs/tutorial/code-signing.md`
- `docs/tutorial/fuses.md`
- `docs/tutorial/asar-integrity.md`
- `docs/tutorial/automated-testing.md`
- `default_app/default_app.ts`
- `lib/browser/guest-window-manager.ts`
- `spec/security-warnings-spec.ts`

与 CSGClaw 直接相关的结论：

1. Electron Main 是窗口和应用生命周期入口，不适合承载产品领域逻辑。
2. Renderer 应保持浏览器模型，默认不拥有 Node 能力。
3. Preload 只能通过 `contextBridge` 暴露逐项、窄化、可校验的能力。
4. 必须显式保持 `contextIsolation`、Renderer Sandbox 和 `webSecurity`。
5. 必须限制导航、新窗口、权限请求和 `shell.openExternal`。
6. IPC Handler 必须校验 sender，不能直接暴露 `ipcRenderer`。
7. 官方安全文档不建议使用高权限的 `file://`，本地静态资源应使用受控协议。
8. 官方推荐 Electron Forge 负责打包和分发。
9. Electron 内置 `autoUpdater` 只覆盖 macOS 和 Windows；Linux 应依赖包管理器或产品自己的更新策略。
10. Electron 安全依赖持续跟进 Electron、Chromium 和 Node 版本，不能长期冻结旧版本。

Electron 仓库自己的 `default_app/default_app.ts` 也明确使用：

```text
contextIsolation: true
sandbox: true
nodeIntegration: false
```

并对 IPC sender、窗口打开和权限请求进行限制。这些应成为 CSGClaw Desktop 的最低基线，而不是可选增强项。

## 3. 三种方案对比

### 3.1 方案 A：Electron 加载 Go Loopback 页面

```text
BrowserWindow
  -> http://127.0.0.1:<dynamic-port>/
     -> Go UI Embed
     -> Go API / SSE / WebSocket
```

优点：

- 完整复用现有 Web UI、相对 API、Hash Router 和 SharedWorker。
- 不增加 CORS、跨 Origin Cookie 和自定义协议调试成本。
- Browser 与 Desktop 仍然共享一套前端行为。
- Go 继续拥有所有业务和持久化能力。
- 可以先做 Desktop Shell，再逐步增加原生能力。

成本：

- 需要正式定义 sidecar 启动、Ready、鉴权和退出契约。
- 需要处理用户已经启动另一个 CSGClaw Server 的情况。
- 必须把当前 `0.0.0.0:18080` 的通用 Server 默认与 Desktop 私有 Listener 分开。

结论：**推荐作为第一阶段和长期默认架构。**

### 3.2 方案 B：Electron 用 Custom Protocol 提供 Renderer，Go 只提供 API

```text
csgclaw://app/
  -> Electron Custom Protocol
  -> React Renderer
  -> http://127.0.0.1:<port>/api
```

优点：

- UI 不依赖 Go 静态文件服务。
- Electron 可以完全控制 Renderer 资源和 CSP。
- 后期可以让 Desktop UI 与 Server UI 使用不同发布节奏。

成本：

- 所有相对 API、SSE SharedWorker、OAuth Return URL、附件 URL 和跳转都要支持可配置 Base URL。
- 需要处理 CORS、Origin、请求鉴权和 Custom Protocol 的 Fetch/Worker 权限。
- Web 和 Desktop 很容易产生两套启动与资源规则。
- 会在当前前端架构拆分之前引入第二个部署边界。

结论：**仅作为未来有明确独立 Desktop UI 版本需求时的第二阶段选项。**

### 3.3 方案 C：把 Go 能力改写成 Electron Main/Node IPC

优点：

- 表面上可以减少本机 HTTP。

问题：

- Agent、Runtime、Sandbox、Channel、Task 和持久化都需要重写或建立大量 IPC。
- CLI/Web 与 Desktop 会形成两套业务实现。
- Electron Main 会成为新的全局业务中心。
- 无法合理复用当前 Go 测试、服务和 release bundle。

结论：**不采用。**

## 4. 目标系统架构

### 4.1 目标进程图

```text
+------------------------------- Operating System ------------------------------+
|                                                                                |
|  +-------------------------- Electron Main ---------------------------------+ |
|  |                                                                          | |
|  |  AppLifecycle          WindowManager          DesktopUpdater              | |
|  |  - single instance     - BrowserWindow        - check/download           | |
|  |  - tray/menu           - navigation policy    - signed whole-app update  | |
|  |  - quit policy         - permission policy    - restart coordination     | |
|  |          |                                                               | |
|  |          v                                                               | |
|  |  SidecarSupervisor                                                        | |
|  |  - locate bundled Go executable                                           | |
|  |  - spawn / ready handshake / logs                                         | |
|  |  - crash recovery / graceful shutdown                                     | |
|  +----------+------------------------------------+---------------------------+ |
|             | child process                    | narrow IPC                   |
|             v                                  v                              |
|  +--------------------------+       +-------------------------------+         |
|  | CSGClaw Go Sidecar       |       | Preload DesktopBridge         |         |
|  |                          |       | runtimeInfo / updater only    |         |
|  | internal/app Bootstrap   |       +---------------+---------------+         |
|  | Domain Services          |                       |                         |
|  | Runtime / Sandbox        |                       v                         |
|  | Local Store              |       +-------------------------------+         |
|  | HTTP UI / API / SSE      +------>| Sandboxed React Renderer      |         |
|  +--------------------------+ HTTP  | existing web/app              |         |
|                                       +-------------------------------+         |
|                                                                                |
|  External Browser <---- HTTPS OAuth / external documentation -----------------+ |
+--------------------------------------------------------------------------------+
```

### 4.2 依赖方向

```text
Electron Main
  -> Desktop lifecycle ports
  -> Sidecar process contract

React Renderer
  -> existing Go HTTP contract
  -> narrow DesktopBridge contract

Go Sidecar
  -> internal/app
  -> domain/application services
  -> infrastructure
```

禁止：

```text
Electron Main -> Agent/Task/Room domain logic
React Renderer -> Node.js / filesystem / child_process
Go domain package -> Electron-specific types
business page -> raw ipcRenderer
Desktop package -> duplicate web/app source tree
```

### 4.3 运行时所有权

| 对象                 | Owner                     | 说明                                       |
| -------------------- | ------------------------- | ------------------------------------------ |
| Electron 进程        | OS / Desktop Launcher     | 单实例                                     |
| BrowserWindow        | Electron Main             | Renderer 崩溃可重建                        |
| Go sidecar           | SidecarSupervisor         | 只管理自己创建的进程                       |
| Agent Runtime        | Go sidecar                | Electron 不直接管理 Sandbox                |
| 业务状态             | Go Services / Local Store | Renderer 和 Main 不复制                    |
| Query Cache          | React Renderer            | 页面展示缓存                               |
| Desktop update state | Electron Main             | 通过窄 Bridge 投影给 Renderer              |
| 用户配置             | `~/.csgclaw`              | CLI/Web/Desktop 共用时必须有单进程写入保护 |

## 5. 目录与工程组织

建议在 `csgclaw` 仓库根目录新增独立的 `desktop/`：

```text
desktop/
  package.json
  pnpm-lock.yaml
  tsconfig.json
  forge.config.ts

  src/
    main/
      main.ts
      appLifecycle.ts
      windowManager.ts
      navigationPolicy.ts
      permissionPolicy.ts
      ipcHandlers.ts
      updater.ts

      sidecar/
        SidecarSupervisor.ts
        contract.ts
        locateBundle.ts
        readyProtocol.ts
        shutdown.ts

    preload/
      index.ts
      desktopBridge.ts
      desktopBridge.types.ts

  resources/
    icons/
    entitlements/

  tests/
    main/
    preload/
    e2e/
```

不建议把 Electron 依赖加到 `web/app/package.json`：

- `web/app` 是 Browser/Renderer 产品代码。
- `desktop` 是 Electron Main/Preload 和打包工程。
- 两者使用不同运行时权限和测试方式。
- Electron 不应污染普通 Web 构建。

建议新增 Make 入口：

```text
make desktop-install
make desktop-dev
make desktop-test
make desktop-package
make desktop-make
```

`make build` 暂时保持现有 CLI/Web 语义，不在默认构建中强制下载 Electron。Desktop 产物成熟后，再决定是否加入 `build-all` 或独立 release workflow。

## 6. Go Sidecar 启动契约

### 6.1 不直接复用当前 `csgclaw serve` 文本输出

当前 `serve` 面向人类和 CLI：

- 打印 Effective Config。
- 可自动打开浏览器。
- 固定从配置解析 Listener。
- `OnReady` 不返回实际 Listener 地址。
- 生产版本会判断官方 bundle 并启用 Go 自升级。
- 前台和 daemon 的进程所有权不同。

Electron 需要机器可验证的稳定契约。建议新增隐藏入口：

```text
csgclaw _desktop-serve
```

或者在完成 `internal/app` 抽取后，由一个专用 Desktop command 调用同一 Bootstrap。不要在 Electron 中拼装几十个 `serve` flag 来模拟桌面模式。

### 6.2 Ready Protocol

建议 sidecar 在成功监听并完成最小初始化后，向 stdout 输出一行 JSON：

```json
{
  "type": "csgclaw.desktop.ready",
  "protocol_version": 1,
  "instance_id": "random-instance-id",
  "pid": 12345,
  "base_url": "http://127.0.0.1:49321",
  "version": "v0.5.0",
  "distribution": "electron"
}
```

规则：

- stdout 只承载版本化控制消息。
- 普通日志写 stderr 和 Desktop 日志文件。
- Ready 消息不得包含 access token。
- Electron 必须设置启动超时并校验 JSON Schema。
- Electron 必须校验 `127.0.0.1`、协议版本、Go 版本和启动 `instance_id`。
- Listener 真正成功后再发送 Ready，不能先找空闲端口再交给另一个进程监听。

后端需要支持：

```text
net.Listen("tcp", "127.0.0.1:0")
  -> 获得实际端口
  -> 用实际 Base URL 构造 OAuth Callback 和内部 Wiring
  -> 发布 Ready
  -> 开始接受请求
```

这要求 `internal/server.Run` 接受预先创建的 Listener，或把实际 Listener 地址传给正式的 Ready Callback。

### 6.3 Session Token

Desktop 启动时生成至少 256 bit 随机 token：

```text
Electron Main
  -> stdin control pipe 的首条 bootstrap message
     -> Go Desktop API Auth Middleware

Electron Session
  -> exact local origin request
     -> inject Authorization: Bearer <desktop-session-token>
```

Bootstrap Control Message 示例：

```json
{
  "type": "csgclaw.desktop.bootstrap",
  "protocol_version": 1,
  "instance_id": "random-instance-id",
  "session_token": "random-session-token"
}
```

要求：

- Token 不出现在命令行参数、环境变量、URL、stdout、Renderer JavaScript 和普通日志中。
- Go 必须先收到并校验 Bootstrap Message，再启动应用和发布 Ready。
- Electron 只对 Ready Protocol 返回的精确 Origin 注入 Header。
- Go Desktop 模式对控制 API、SSE 和 WebSocket 统一验证 Token。
- `/healthz` 可以只返回最小健康状态，不返回版本、路径或配置。
- OAuth Callback 使用它自己的 state/callback 校验，不依赖 Renderer Token。
- Browser/CLI 模式保留现有鉴权兼容；强化规则先限定在 Desktop Mode，避免一次改动所有客户端。

### 6.4 单进程写入与已有 Server

Electron 单实例锁只能防止第二个 Electron，不能防止用户同时运行：

```text
csgclaw serve
Electron -> bundled csgclaw _desktop-serve
```

两个 Go 进程共享 `~/.csgclaw` 会带来：

- State JSON 并发写。
- Agent Runtime 重复启动。
- Manager 和 Channel Bridge 重复运行。
- Upgrade、migration 和 PID 状态冲突。

因此 Go Application Bootstrap 必须增加共享实例锁：

```text
~/.csgclaw/runtime.lock
```

建议行为：

1. 第一个 Go Application 实例持有锁。
2. 第二个实例返回结构化 `already_running` 错误。
3. Electron 不自动附着到任意 `18080` 服务。
4. 只有已有服务提供兼容的 Desktop Handshake、版本和安全策略时，才能进入显式 Attach 模式。
5. MVP 可以直接提示用户停止已有 `csgclaw serve`，不要为赶进度绕过 Store Lock。

### 6.5 优雅停止与崩溃恢复

推荐使用 Electron 与 sidecar 之间的 stdin 控制通道：

```json
{ "type": "csgclaw.desktop.shutdown", "reason": "app-quit" }
```

行为：

- 显式 Quit：发送 shutdown，等待 Go 关闭 HTTP、Bridge、Agent Service 和文件句柄。
- 超时：先发送平台温和终止信号，最后才强制结束。
- Electron Main 崩溃：stdin pipe EOF，Go 启动短暂 grace period 后自行退出。
- Renderer 崩溃：只重建 BrowserWindow，不重启 Go。
- Go 崩溃：展示本地 Recovery Window，提供重试、查看日志、退出。
- Electron 不得结束不是自己创建的外部 CSGClaw 进程。

SidecarSupervisor 至少维护：

```text
idle
  -> starting
  -> ready
  -> stopping
  -> stopped

starting/ready
  -> crashed
  -> retrying
  -> ready | failed
```

连续崩溃必须有退避和最大次数，不能无限重启。

## 7. Electron Main 与 BrowserWindow

### 7.1 安全配置

建议显式配置，不依赖默认值：

```ts
new BrowserWindow({
  show: false,
  webPreferences: {
    preload,
    contextIsolation: true,
    sandbox: true,
    nodeIntegration: false,
    nodeIntegrationInWorker: false,
    nodeIntegrationInSubFrames: false,
    webSecurity: true,
    allowRunningInsecureContent: false,
    experimentalFeatures: false,
    webviewTag: false,
  },
});
```

同时：

- 在 `ready` 前调用 `app.enableSandbox()`。
- 使用独立持久 Session Partition。
- Renderer 不允许 `remote`、raw `ipcRenderer` 或任意 Node Global。
- 生产环境默认关闭 DevTools 快捷入口；保留显式诊断开关。
- `ready-to-show` 后再显示主窗口，避免空白闪烁。

### 7.2 导航和外部链接

主窗口只允许：

```text
http://127.0.0.1:<ready-port>/*
```

以下事件都要建立策略：

- `will-navigate`
- `will-redirect`
- `setWindowOpenHandler`
- `will-attach-webview`

默认处理：

1. 同一精确 Origin 的内部导航允许。
2. `file:`、`javascript:`、`data:`、任意 shell scheme 拒绝。
3. 普通 `https:` 外链在验证 URL 后交给系统浏览器。
4. OAuth URL 通过专用 `openOAuth` 能力处理，不暴露任意 `openExternal(url)`。
5. 新窗口默认 `deny`，不在 Electron 内创建任意远端 BrowserWindow。

自定义 OpenCSG 环境可能使用不同域名，因此 OAuth 不能只写死生产域。应校验：

- 必须是 `https:`。
- URL 不含用户名密码。
- Host 与本次用户选择的认证环境或后端签发的 OAuth Start Response 对应。
- 非已知 Host 第一次打开时要求用户确认。

### 7.3 权限策略

为 Desktop Session 同时配置：

- `setPermissionRequestHandler`
- `setPermissionCheckHandler`

默认拒绝全部权限。只有产品明确需要后，才逐项开放：

- `notifications`
- `clipboard-sanitized-write`
- 未来可能的 `display-capture`

相机、麦克风、地理位置、MIDI、USB、Serial、HID、File System Access 等不得因为 Chromium 默认行为而自动放行。

### 7.4 CSP 与 HTTP 安全头

当前 `web/app/index.html` 含两段 inline bootstrap script，Go UI Handler 也未设置 CSP。Desktop 上线前需要：

1. 把 Theme 和 Base URL inline script 移到构建模块，或为它们生成 nonce/hash。
2. 在 Go UI Response 增加经过测试的 CSP。
3. 增加：
   - `X-Content-Type-Options: nosniff`
   - `Referrer-Policy`
   - `frame-ancestors 'none'`
   - 合理的 `Permissions-Policy`
4. 不用全局 `'unsafe-inline'` 作为长期解决方案。
5. CSP 需要覆盖 Mermaid、Markdown 图片、Blob 附件、SharedWorker 和实际 API 连接场景。

Loopback HTTP 是为了连接本机 Go sidecar 的受限例外：

- 只使用数字地址 `127.0.0.1`。
- 不使用 `0.0.0.0`、局域网 IP 或可被解析变化影响的 Host。
- 使用动态高位端口、随机 token、Host/Origin 校验。
- 所有远程资源仍必须使用 HTTPS/WSS。

## 8. Preload 与 DesktopBridge

### 8.1 最小 API

建议第一阶段只暴露：

```ts
type DesktopBridge = {
  getRuntimeInfo(): Promise<{
    platform: "darwin" | "win32" | "linux";
    arch: string;
    appVersion: string;
    backendVersion: string;
  }>;

  openOAuth(input: {
    purpose: "opencsg-auth" | "github-connector";
    url: string;
  }): Promise<{ opened: boolean }>;

  checkForUpdates(): Promise<void>;
  installDownloadedUpdate(): Promise<void>;
  onUpdateStatus(listener: (status: DesktopUpdateStatus) => void): () => void;
};
```

不暴露：

```text
ipcRenderer.send
ipcRenderer.invoke
shell.openExternal
fs
child_process
process.env
任意 path 读写
```

### 8.2 IPC 校验

每一个 Main Handler 必须：

- 校验 `event.sender` 是当前主窗口的 `webContents`。
- 校验 `senderFrame` 是 Main Frame。
- 校验 URL Origin 等于当前 sidecar Origin。
- 使用 Zod 或等价 Schema 校验参数。
- 使用固定 IPC Channel 常量。
- 返回稳定、可序列化、无 Electron 对象的 DTO。
- 不把 IPC Event 传回 Renderer callback。

### 8.3 Web 平台适配层

不要在 Page 或 Workspace Controller 中直接判断 Electron。建议新增：

```text
web/app/src/shared/platform/
  runtime.ts
  browserRuntime.ts
  desktopRuntime.ts
  externalNavigation.ts
  updatePort.ts
```

统一抽象：

```text
ExternalNavigationPort
  browser -> location.assign / window.open
  desktop -> DesktopBridge.openOAuth / system browser

UpdatePort
  browser -> Go upgrade API
  desktop -> Electron autoUpdater
```

这与前端架构评估中的方向一致：

- 平台副作用不进入纯 Model。
- Desktop 能力不进入 `useWorkspaceController`。
- Route Page 和 Shell Feature 只依赖窄 Port。

## 9. OAuth 与外部浏览器

### 9.1 当前问题

当前两条链路使用不同浏览器行为：

```text
OpenCSG Login
  -> window.location.assign(login_url)

GitHub Connector
  -> 先 window.open(about:blank)
  -> 异步获取 authorization_url
  -> 修改 popup.location
```

在 Electron 中：

- 主窗口不能导航到远端认证页面。
- 新建远端 Electron Window 会扩大攻击面。
- `about:blank` 预开窗口会与默认 deny popup 策略冲突。
- OAuth Callback 后不应把系统浏览器停在完整 CSGClaw Web UI。

### 9.2 目标流程

```text
Renderer
  -> POST Go OAuth Start
  -> 获得 HTTPS authorization_url
  -> DesktopBridge.openOAuth(purpose, url)
  -> System Browser
  -> Provider
  -> Go loopback OAuth Callback
  -> Go 保存 credential
  -> 返回轻量 "Login complete, return to CSGClaw" 页面
  -> Renderer 继续轮询 auth/connector status
```

第一阶段继续复用 loopback Callback，不急于引入 `csgclaw://` Deep Link。Deep Link 需要额外处理：

- OS Scheme 注册。
- 单实例转发。
- 安装器注册和卸载。
- OAuth Provider Redirect 白名单。
- URL 参数安全校验。

只有当浏览器 Callback 用户体验不能接受时，再单独设计 Deep Link。

## 10. Desktop 生命周期

### 10.1 单实例

Electron 启动最早阶段调用：

```text
app.requestSingleInstanceLock()
```

第二个实例：

- 不启动第二个 Go sidecar。
- 把参数传给第一个实例。
- 恢复、显示并聚焦主窗口。

### 10.2 关窗口与 Quit

CSGClaw 存在长时间 Agent 工作，窗口关闭不能等价于强制停止所有 Runtime。

推荐语义：

| 操作                   | 行为                                        |
| ---------------------- | ------------------------------------------- |
| macOS 关闭窗口         | 保留 App 和 sidecar，Dock 再次点击重建窗口  |
| Windows/Linux 关闭窗口 | 默认隐藏到托盘，sidecar 继续运行            |
| 托盘“打开 CSGClaw”     | 显示或重建主窗口                            |
| 托盘“退出”             | 检查活跃工作，确认后优雅停止 sidecar 并退出 |
| 系统关机/注销          | 尽力优雅停止，不阻塞系统退出过久            |
| 安装更新               | 停止 sidecar，确认退出后再执行安装          |

如果 MVP 暂时不做托盘，则必须在关闭窗口时明确提示：

```text
退出会停止本机 CSGClaw 服务和正在运行的 Agent。
```

不能静默强杀。

### 10.3 日志与恢复

日志分层：

```text
Electron Main Log
Electron Updater Log
Go server.log
Agent/Runtime existing logs
```

Electron 应使用 `app.setAppLogsPath()` 建立平台标准日志目录。Recovery UI 至少提供：

- 重试启动。
- 打开日志目录。
- 复制错误摘要。
- 检查已有 CSGClaw 进程。
- 退出。

Renderer 加载失败时不能只显示白屏。

## 11. 打包结构

### 11.1 复用完整 Go Release Bundle

不要只拷贝一个 `csgclaw` binary。当前正式 bundle 还包含：

```text
csgclaw/
  .csgclaw-bundle.json
  bin/
    csgclaw[.exe]
    csgclaw-cli[.exe]
    boxlite[.exe]
    sandbox-tools/
      csgclaw-cli
```

Electron `extraResources` 应嵌入完整、已校验的 bundle：

```text
CSGClaw Desktop.app/
  Contents/
    Resources/
      app.asar
      backend/
        csgclaw/
          .csgclaw-bundle.json
          bin/
            ...
```

Windows/Linux 使用等价的 `resources/backend/csgclaw`。

原因：

- 保持 `validateServeInstallation()` 对官方 bundle 的判断。
- 保持 BoxLite 和 Sandbox CLI 装配。
- 保持宿主 `csgclaw-cli` 能力。
- 避免 Electron 打包脚本复制一套新的 Go bundle 规则。

Go bundle 必须位于 ASAR 外：

- 它需要可执行权限。
- Runtime 需要访问伴生 binary。
- Electron app code 使用 ASAR Integrity；sidecar 由 OS 代码签名和整包签名保护。

### 11.2 构建输入

建议新增可被 Electron Forge 消费的 staging target：

```text
make desktop-backend-bundle GOOS=<target> GOARCH=<target>
```

输出固定目录，不直接让 Forge 解压 GitHub Release：

```text
dist/desktop-input/<goos>-<goarch>/csgclaw/
```

构建顺序：

```text
web/app build
  -> web/static-dist
  -> Go release bundle
  -> Desktop main/preload build
  -> Forge package
  -> Forge maker
  -> sign/notarize
  -> smoke test
  -> publish
```

Web UI 只嵌入 Go binary 一次，不在 Electron ASAR 中再复制一份。

### 11.3 平台产物

推荐第一阶段：

| 平台            | 产物                       | 更新                                    |
| --------------- | -------------------------- | --------------------------------------- |
| macOS arm64/x64 | DMG + ZIP                  | Squirrel.Mac / Electron autoUpdater     |
| Windows x64     | Squirrel Installer         | Squirrel.Windows / Electron autoUpdater |
| Linux x64/arm64 | deb，按需增加 rpm/AppImage | 包管理器或手动下载                      |

Windows 企业 MSI/MSIX 可以后续增加，不要在 MVP 同时维护多个安装技术。

## 12. 代码签名、Fuses 与供应链

### 12.1 macOS

发布必须：

1. 签名 Go sidecar、宿主 `csgclaw-cli`、BoxLite 等嵌套 Mach-O。
2. 签名 Electron Helpers 和主 App。
3. 使用 Hardened Runtime 和经过验证的 Entitlements。
4. 提交 Apple Notarization。
5. Staple 公证结果。
6. 在干净机器验证 Gatekeeper 启动、Sandbox Runtime 和升级。

Sandbox 内使用的 Linux `sandbox-tools/csgclaw-cli` 不是 Mach-O，不单独做 Apple Code Sign；它必须被 App Resource Seal、发布校验和 checksum 覆盖。

如果 BoxLite 需要额外虚拟化或系统权限，必须通过实际签名包验证；不要假设普通 Electron Entitlement 足够。

### 12.2 Windows

发布必须：

- 签名 Go PE、宿主 CLI、Electron EXE、Updater 和 Installer。
- 配置稳定的 App User Model ID。
- 在干净 Windows VM 验证 SmartScreen、安装、卸载、升级和进程清理。
- 签名凭据只进入受保护 CI，不写入仓库或构建日志。

Linux Sandbox CLI 等非 PE 资源通过安装包签名、hash 和发布 checksum 校验，不作为 Windows 可执行文件签名。

### 12.3 Electron Fuses

建议使用 Forge Fuses Plugin：

| Fuse                                    | 建议               |
| --------------------------------------- | ------------------ |
| `RunAsNode`                             | 禁用               |
| `EnableNodeOptionsEnvironmentVariable`  | 禁用               |
| `EnableNodeCliInspectArguments`         | 禁用               |
| `EnableCookieEncryption`                | 启用               |
| `EnableEmbeddedAsarIntegrityValidation` | macOS/Windows 启用 |
| `OnlyLoadAppFromAsar`                   | 启用               |
| `GrantFileProtocolExtraPrivileges`      | 禁用               |

启用 ASAR Integrity 时必须保持 Forge/Packager 版本满足官方最低要求，并在打包后读取 Fuse 状态做 CI 验证。

### 12.4 版本和依赖

- Electron 使用精确版本，不使用宽松 Major Range。
- 每个 Electron Stable Major 都评估 Breaking Changes。
- 安全修复不等待普通功能发布周期。
- 锁定 Forge、Maker、Fuses 和签名工具版本。
- 生成 SHA256、SBOM 和构建来源信息。
- Desktop 版本、Go `version.Current()` 和 release tag 来自同一个版本输入。

## 13. 升级所有权

### 13.1 不能同时存在两个自升级器

当前 Go binary 在官方 bundle 内会认为自身支持升级。如果直接嵌入 Electron Resources：

- Go Upgrade Helper 可能尝试替换签名 App 内部的 `resources/backend`。
- 更新后 Electron 版本和 Go 版本可能不一致。
- macOS 代码签名会失效。
- Electron Updater 与 Go Updater 会产生竞态。

因此 Desktop Mode 必须明确：

```text
distribution = electron
go_auto_upgrade = disabled
desktop_whole_app_update = enabled
```

Go Upgrade Status 返回：

```json
{
  "auto_upgrade_supported": false,
  "auto_upgrade_unsupported_reason": "desktop_managed"
}
```

React 的 `UpdatePort` 在 Desktop 中使用 Electron Updater，不再调用 Go `upgrade/apply`。

CLI/Web 安装继续使用现有 Go Upgrade，不受影响。

### 13.2 更新流程

```text
Electron Main
  -> check update
  -> download signed Desktop package
  -> notify Renderer
  -> user selects restart
  -> block new destructive actions
  -> gracefully stop owned Go sidecar
  -> autoUpdater.quitAndInstall()
  -> new Desktop starts matching Go bundle
```

要求：

- Go 与 Electron 进行 version/contract handshake。
- Major contract 不兼容时禁止加载主窗口。
- 更新失败不能损坏 `~/.csgclaw`。
- Migration 由新 Go sidecar 在获得实例锁后执行。
- Linux 不伪装支持 Electron 内置 autoUpdater。

## 14. 与现有架构优化的关系

### 14.1 后端

Electron 改造依赖后端架构评估的“阶段 2：正式 Application Bootstrap”，但不需要等待全部后端重构完成。

建议把 Desktop 作为 Bootstrap 抽取的第一个真实消费者：

```text
CLI serve adapter --------+
                          |
Electron sidecar adapter -+-> internal/app Bootstrap
                          |
tests --------------------+
```

最小抽取范围：

- Migration。
- Provider Preflight。
- Service Construction。
- Recovery。
- Runtime Registration。
- Server Lifecycle。
- Graceful Shutdown。

`cli/serve` 保留：

- Flag。
- daemon/PID。
- 人类输出。
- 自动打开浏览器。

Desktop Adapter 保留：

- Ready Protocol。
- Parent Control Channel。
- Desktop Session Token。
- Electron Distribution Mode。

### 14.2 前端

Electron 改造不应阻塞 Workspace 拆分，也不应让 Desktop 状态进入 Workspace 大 Controller。

建议顺序：

```text
先建立 shared/platform 和 DesktopBridge contract
  -> 适配 OAuth / Upgrade
  -> Electron MVP

Workspace Route Controller 拆分可并行推进
  -> 最终把 Update Feature 放到 Shell
  -> 各 Page 不感知 Electron
```

### 14.3 Mantine

Mantine 迁移与 Electron 进程架构独立：

- Mantine 仍位于 `components/ui` 后面。
- Electron 不直接依赖 Mantine。
- Portal、Dialog 和 Focus Trap 需要在 BrowserWindow 中做一轮桌面回归。
- AppShell/Sidebar 仍按 Mantine 方案放到后期。

## 15. 分阶段实施计划

### 阶段 0：技术验证与契约冻结

目标：在不发布安装包的前提下证明架构。

任务：

1. 新建 `desktop/` 最小 Electron Main/Preload。
2. 抽取最小 `internal/app` Bootstrap。
3. 实现 `_desktop-serve`、动态端口和 Ready JSON。
4. Electron 启动本地 Go build 并加载 UI。
5. 验证 API、SSE SharedWorker、WebSocket 和附件。
6. 验证 Renderer 无 Node 权限。
7. 记录启动时间、内存和安装体积基线。

完成标志：

- macOS arm64 开发环境可以一条命令启动。
- Conversation、Agent、Task 等主流程可用。
- 关闭 Renderer 不会误杀 Go；显式 Quit 能优雅停止。

### 阶段 1：安全与平台适配

任务：

1. Desktop Session Token 和统一 API Auth。
2. Host/Origin 校验。
3. Navigation、Window Open 和 Permission Policy。
4. CSP 和安全头。
5. `DesktopBridge` sender/schema 校验。
6. OAuth External Navigation Adapter。
7. Desktop UpdatePort，禁用 Go 自升级。
8. Go Application 实例锁。

完成标志：

- 安全测试不能从 Renderer 获取 Node、Token 或任意 IPC。
- OAuth 登录和 GitHub Connector 在系统浏览器完成。
- 同时启动 CLI Server 和 Desktop 不会并发写 Store。

### 阶段 2：macOS 可发布版本

任务：

1. Forge DMG + ZIP。
2. 完整 Go bundle `extraResources`。
3. 嵌套可执行文件签名。
4. Hardened Runtime、公证和 Staple。
5. macOS arm64/x64 CI。
6. Electron whole-app updater。
7. 单实例、托盘、日志和 Recovery UI。

完成标志：

- 干净 macOS 机器可安装、启动、升级和卸载。
- Gatekeeper 无警告。
- 更新后 Electron/Go 版本一致。

### 阶段 3：Windows 可发布版本

任务：

1. Windows x64 Forge Maker。
2. EXE/Installer 签名。
3. Squirrel 启动事件和 App User Model ID。
4. Windows child process、EOF Shutdown 和强制退出回归。
5. Windows 安装、升级和卸载 CI/VM 测试。

完成标志：

- 无遗留 sidecar 进程。
- 升级不丢配置、不重复启动 Agent。
- SmartScreen 和安装器体验满足发布标准。

### 阶段 4：Linux 与桌面体验增强

任务：

- deb/rpm/AppImage 取舍。
- 包管理器升级提示。
- Tray、Notification、Badge、Protocol Link。
- Native File Dialog 和 Reveal in Finder/Explorer。
- Crash Report 和诊断包。

不要在阶段 0 同时实现这些能力。

## 16. 测试与验收

### 16.1 Go

至少增加：

- `127.0.0.1:0` 实际地址测试。
- Ready Protocol Schema/时序测试。
- Desktop Token 正确、缺失、错误测试。
- OAuth Callback 例外路径测试。
- stdin shutdown/EOF 测试。
- Application 实例锁测试。
- `desktop_managed` Upgrade 测试。
- Desktop/CLI Bootstrap 等价性测试。

### 16.2 Electron Main/Preload

单元测试：

- Bundle Path Resolution。
- Ready JSON 解析。
- 超时、崩溃、重试状态机。
- Navigation URL Policy。
- OAuth Host Validation。
- IPC Sender 和 Payload Validation。
- Update State Projection。

### 16.3 Renderer

保留现有 Vitest/RTL，并增加：

- BrowserRuntime 与 DesktopRuntime Contract Test。
- OpenCSG OAuth 不再强制 Electron 主窗口跳转。
- GitHub Connector 不依赖 `about:blank` Popup。
- Browser Upgrade 与 Desktop Upgrade 分流。
- Desktop Bridge 不存在时普通浏览器行为不变。

### 16.4 E2E

Electron 官方文档列出的 WebdriverIO 和 Playwright 都可用于 E2E。建议：

- 使用 Playwright Electron API 做开发态快速 E2E。
- 使用打包后 Smoke Script 验证真实安装产物。
- 不把开发态 Electron 测试等同于签名安装包验收。

关键场景：

1. 首次启动并自动完成本地 Bootstrap。
2. 第二实例聚焦已有窗口。
3. 已有 CLI Server 时给出明确错误。
4. OAuth 系统浏览器往返。
5. Renderer Crash 后重建。
6. Go Crash 后 Recovery。
7. 有 Agent 工作时关闭窗口和显式 Quit。
8. 下载并安装 Desktop Update。
9. 网络离线时仍能启动本地 UI，并明确显示远程能力错误。
10. Windows/macOS 干净用户环境安装与卸载。

### 16.5 安全验收

- Renderer 中 `require`、`process`、`fs` 不可用。
- 任意 iframe/child frame 不能调用 Desktop IPC。
- 任意非本地 Origin 导航被阻止。
- `window.open` 不会创建任意远端 Electron Window。
- 任意非 HTTPS OAuth URL 被拒绝。
- Permission 默认拒绝。
- Desktop Token 不在 URL、Renderer Storage 和日志中。
- 修改 `app.asar` 后 macOS/Windows 应拒绝运行。
- Electron Fuses 与预期一致。
- Go sidecar 和嵌套工具均包含在签名检查中。

## 17. 风险与应对

| 风险                               | 影响                   | 应对                                           |
| ---------------------------------- | ---------------------- | ---------------------------------------------- |
| Electron 包体明显增大              | 下载和更新成本上升     | 单份 Web UI、按平台产物、测量增量更新          |
| Electron 与 Go 双进程启动慢        | 首屏体验下降           | Ready 分阶段、启动页、延迟非关键 Agent 恢复    |
| Desktop 与 CLI 同时运行            | Store 损坏、重复 Agent | Go Application 实例锁                          |
| 动态端口影响 OAuth                 | Callback 错误          | Listener 后计算实际 Advertise URL              |
| Renderer XSS 提权                  | 本机安全风险           | Sandbox、Context Isolation、窄 IPC、CSP、Token |
| Go 自升级修改签名 App              | 包损坏、版本错配       | Desktop Mode 禁用 Go Upgrade                   |
| Electron 长期不升级                | Chromium 漏洞          | 固定升级节奏和安全发布通道                     |
| Windows Child Process 难以优雅停止 | 残留进程和数据损坏     | stdin Control、超时、VM E2E                    |
| macOS 嵌套 Binary 签名复杂         | 公证失败               | 固定签名顺序、干净机 Gatekeeper 验证           |
| Linux 更新能力不一致               | 用户体验差异           | 明确依赖包管理器，不伪装跨平台一致             |
| OAuth Popup 行为改变               | 登录失败               | Platform Adapter + 系统浏览器 Contract Test    |
| 当前 CSP 缺失且有 inline script    | XSS 风险               | 抽取 inline bootstrap，建立 CSP 回归           |

## 18. 不建议做的事情

- 不把 Electron 代码放进 `web/app/src`。
- 不允许 Renderer 使用 `nodeIntegration: true`。
- 不关闭 `contextIsolation`、Sandbox 或 `webSecurity`。
- 不直接向 Renderer 暴露 `ipcRenderer`、`shell`、`fs` 或 `child_process`。
- 不把任意 Renderer URL 直接传给 `shell.openExternal`。
- 不使用固定 `18080` 作为 Desktop 私有端口。
- 不让 Desktop 默认监听 `0.0.0.0`。
- 不在 URL 中传 Desktop Token。
- 不自动附着到身份和版本未知的已有 CSGClaw Server。
- 不让 Go Updater 和 Electron Updater 同时工作。
- 不只拷贝一个 Go binary 而遗漏 bundle 中的伴生工具。
- 不把 Go sidecar 放进 ASAR。
- 不在一个 CI Runner 上假装完成所有平台签名。
- 不因为做 Electron 而暂停前后端现有架构边界治理。
- 不在 MVP 同时实现托盘、Deep Link、自动更新、原生文件系统和多窗口全部能力。

## 19. 最终建议顺序

```text
1. 抽取最小 internal/app Bootstrap
2. 建立 Desktop Sidecar Contract
3. Electron 加载 Go Loopback UI
4. 完成 Token / Navigation / IPC / CSP 安全闭环
5. 适配 OAuth 和 Desktop UpgradePort
6. 做 macOS 签名安装包
7. 做 Windows 安装和升级
8. 最后扩展 Linux 与原生桌面体验
```

最关键的架构判断不是“Electron 选哪个脚手架”，而是守住三条边界：

```text
Electron 管桌面，不管业务
Go 管业务，不自改 Desktop 包
React 管界面，不获得本机高权限
```
