# CSGClaw Mantine 采用与渐进迁移方案

> - 评估日期：2026-07-24
> - 代码基线：`f6559fed`（`upstream/main`）
> - 范围：`web/app` UI 基础组件、主题、样式层和布局组件
> - 结论性质：UI 基础设施选型与迁移方案，不包含本次代码实施

关联文档：[前端架构评估](architecture-assessment.zh.md)

## 1. 总结

### 1.1 一句话结论

Mantine 可以支持当前 CSGClaw UI，综合匹配度约为 **8.5/10**，适合采用，但必须渐进迁移。

> Mantine 提供底层完整组件能力；CSGClaw 保留自己的 Token、视觉规范和本地组件 API。

不建议：

- 一次性把所有 Radix/原生组件改写成 Mantine。
- 让业务页面直接大规模导入 `@mantine/core`。
- 直接采用 Mantine 默认视觉。
- 在 Workspace/App Shell 架构拆分前先重写整个 Sidebar 和布局。

建议：

- 保留 `src/components/ui` 作为 CSGClaw 的稳定 UI 边界。
- 在本地 UI 组件内部逐个把 Radix/原生实现替换为 Mantine。
- 允许 Mantine 与 Radix 在迁移期并存。
- 最后再决定能否删除 `radix-ui` 依赖。

### 1.2 核心决策

| 决策          | 处理方式                                               |
| ------------- | ------------------------------------------------------ |
| Mantine 定位  | UI 实现基础设施，不是产品视觉规范                      |
| CSGClaw Token | 继续作为品牌和语义样式真相                             |
| 本地 UI API   | 保留 `components/ui`，业务层不感知底层替换             |
| 迁移模式      | 按组件渐进迁移，Mantine/Radix 临时并存                 |
| Tailwind      | 保留，使用 Mantine CSS Layer 控制优先级                |
| 图标          | 继续使用 Lucide，不因 Mantine 迁移替换                 |
| 表单          | 继续使用 React Hook Form，不强制引入 Mantine Form      |
| 业务组件      | Conversation、Agent、File Tree、Activity Card 继续自研 |
| AppShell      | 最后迁移，并与 Workspace 架构拆分协调                  |

## 2. 兼容性与当前基础

### 2.1 版本兼容

| 项目              | 当前版本/要求    | 结论                                |
| ----------------- | ---------------- | ----------------------------------- |
| CSGClaw React     | `19.2.6`         | 满足 Mantine 要求                   |
| CSGClaw React DOM | `19.2.6`         | 满足 Mantine 要求                   |
| CSGClaw Vite      | `8.0.13`         | 可正常接入                          |
| CSGClaw Tailwind  | `4.3.0`          | 可与 CSS Layer 共存                 |
| CSGClaw Radix     | `radix-ui 1.6.1` | 迁移期保留                          |
| Mantine Core      | `9.2.1`          | Peer 要求 React/React DOM `^19.2.0` |
| Mantine Hooks     | `9.2.1`          | 与 Core 版本一致                    |

代码依据：

- `web/app/package.json`

### 2.2 当前 CSGClaw UI 基础

当前本地 UI 层包含：

```text
components/ui/
  AppLayout
  Button
  Dialog
  DropdownMenu
  FormControls
  Icons
  Popover
  Select
  Tooltip
```

在当前快照中：

- 约 67 个 TS/TSX 文件引用 `components/ui`。
- Button 定义了 12 个 CSGClaw Variant 和 5 个 Size。
- Select 支持搜索、选项说明、空值、受控状态和自定义 Portal Container。
- Dialog 支持自定义 Overlay、Portal Container、Focus 策略和 `asChild` Close。
- Tooltip、Popover 和 Dropdown 都封装了 Radix Compound Component API。
- `tokens.css` 已包含颜色、间距、圆角、阴影、字体、Z-Index 和亮暗主题语义变量。

这些能力说明 CSGClaw 已有自己的 UI Contract。Mantine 应替换 Contract 后面的实现，而不是替换 Contract 本身。

### 2.3 Mantine 能力

Mantine 9.2.1 的 `@mantine/core` 提供约 109 个组件，包括：

```text
AppShell / Button / Modal / Drawer / Dialog
Select / Combobox / Menu / Popover / Tooltip
Tabs / Tree / ScrollArea / NavLink
Input / TextInput / NumberInput / FileInput
FocusTrap / Portal / Transition / Overlay
```

Mantine 提供多层自定义能力：

1. `createTheme`：颜色、字体、间距、圆角、阴影和断点。
2. `theme.components`：组件全局默认 Props、Class Names 和 Styles。
3. `classNames` / `styles`：覆盖组件内部 Slot。
4. `vars`：按组件生成 CSS Variables。
5. `variantColorResolver`：映射或增加 Variant。
6. `unstyled`：单组件移除默认样式。
7. `MantineProvider headless`：全局 Headless 模式。
8. `portalProps`：控制 Portal Target 等行为。

## 3. 当前与目标 UI 架构

### 3.1 当前 UI 架构图

```text
+--------------------------- Pages / Business UI -------------------------------+
| Conversation / Agent / Tasks / Hub / Workspace / Settings                     |
+------------------------------------+-------------------------------------------+
                                     |
                                     v
+------------------------ CSGClaw Local UI Contract -----------------------------+
| components/ui                                                                  |
|                                                                                |
| Button       Select       Dialog       Tooltip       Popover       Dropdown    |
+-----+-----------+------------+-------------+-------------+-------------+--------+
      |           |            |             |             |             |
      v           +------------+-------------+-------------+-------------+
 Native DOM                              Radix Compound Components
      |                                          |
      +---------------------+--------------------+
                            |
                            v
+-------------------------- Styling Foundations --------------------------------+
| CSGClaw tokens.css + component CSS + Tailwind utilities                        |
+-------------------------------------------------------------------------------+
```

### 3.2 目标 UI 架构图

```text
+--------------------------- Pages / Business UI -------------------------------+
| 只使用 CSGClaw UI API，不直接依赖具体 UI Library                               |
+------------------------------------+-------------------------------------------+
                                     |
                                     v
+------------------------ CSGClaw Local UI Contract -----------------------------+
| components/ui                                                                  |
|                                                                                |
| Button / Select / Dialog / Tooltip / Popover / Menu / AppLayout                |
| CSGClaw props + variants + accessibility + portal conventions                  |
+----------------------+------------------------------+--------------------------+
                       |                              |
                       | migrated                    | temporary legacy
                       v                              v
+------------------------------+       +----------------------------------------+
| Mantine Adapters             |       | Radix / Native Adapters                |
| @mantine/core + hooks        |       | component not migrated yet             |
+---------------+--------------+       +-------------------+--------------------+
                |                                          |
                +----------------------+-------------------+
                                       |
                                       v
+-------------------------- CSGClaw Theme Bridge -------------------------------+
| MantineProvider                                                               |
| mantineTheme.ts + component defaults + variant resolver                       |
| CSGClaw color scheme bridge + Portal/Z-Index policy                           |
+------------------------------------+------------------------------------------+
                                     |
                                     v
+--------------------------- Styling Foundations -------------------------------+
| Mantine styles.layer.css -> CSGClaw tokens.css/component CSS -> Tailwind       |
| CSGClaw Token 和视觉规范保持最终控制权                                         |
+-------------------------------------------------------------------------------+
```

### 3.3 目标依赖规则

```text
pages/components/business
  -> components/ui
     -> @mantine/core or temporary radix-ui adapter
        -> MantineProvider/theme bridge
           -> CSGClaw tokens and CSS layers
```

禁止：

```text
business pages -> mixed Mantine and CSG wrapper for the same component
Mantine default theme -> overwrite CSGClaw semantic tokens
component migration -> silently change public prop semantics
```

### 3.4 为什么必须保留本地 UI Contract

如果业务页面直接导入 Mantine：

- 67 个现有使用文件会与 Mantine Props 和组件结构耦合。
- Mantine 升级会直接扩散到 Page 和 Business Component。
- CSGClaw 的 12 种 Button Variant 会被 Mantine 默认 Variant 替代。
- 空值、Portal、焦点和 `asChild` 等本地行为难以统一。
- 迁移无法按组件回滚。

保留 `components/ui` 后，每个组件可以独立选择：

```text
Button        -> Mantine
TextInput     -> Mantine
Select        -> Mantine Adapter
Dialog        -> 暂时 Radix
WorkspaceTree -> CSGClaw 自研
```

这使迁移成为内部实现替换，而不是全站 API 改写。

## 4. 能力匹配

| 当前能力                   | Mantine 对应方案                 | 匹配度   | 说明                           |
| -------------------------- | -------------------------------- | -------- | ------------------------------ |
| 12 种 Button Variant       | Button + `variantColorResolver`  | 高       | 需要建立 CSG Variant 映射      |
| 多级颜色、圆角、阴影 Token | `createTheme` + CSS Variables    | 高       | CSG Token 保持真相             |
| 深色/浅色模式              | MantineProvider Color Scheme     | 高       | 需要桥接现有 `data-theme`      |
| 搜索型 Select、复杂选项    | Select/Combobox + `renderOption` | 高       | 需保留空值和现有回调语义       |
| Dialog/Popover/Dropdown    | Modal/Popover/Menu               | 高       | Compound API 不能机械替换      |
| 自定义 Portal Container    | `portalProps.target`             | 高       | 需统一 HTMLElement/string 转换 |
| Tooltip、焦点、键盘交互    | Mantine 内置                     | 高       | 需回归现有特殊 Focus 策略      |
| Sidebar、工作区布局        | AppShell/NavLink/Tabs/ScrollArea | 中高     | 应在 Workspace 拆分后处理      |
| 对话流、文件树、活动卡片   | 保留 CSGClaw 业务组件            | 需要自研 | Mantine 只提供底层 Primitive   |

## 5. Theme 与样式策略

### 5.1 Token 的所有权

`src/shared/styles/tokens.css` 继续是 CSGClaw 的品牌和语义 Token 真相，包括：

- Brand、Gray、Error、Warning 等颜色梯度。
- `--bg`、`--panel`、`--text`、`--line` 等语义色。
- Spacing、Radius、Shadow、Typography。
- Focus Ring、Z-Index、亮暗模式映射。

Mantine Theme 是 Adapter：

```text
CSGClaw design tokens
  -> mantineTheme.ts 映射
  -> Mantine component variables/defaults
  -> local component CSS 做最终视觉约束
```

不要同时在 `tokens.css` 和 `mantineTheme.ts` 手工维护两套独立颜色真相。对于 Mantine 必须使用的颜色数组，应集中在一个映射模块，并建立 Token 对照测试或视觉基线。

建议目录：

```text
src/shared/theme/
  mantineTheme.ts
  mantineComponents.ts
  variantColorResolver.ts
  colorSchemeBridge.tsx
  portalPolicy.ts

src/shared/styles/
  tokens.css
  globals.css
  mantine-overrides.css
```

### 5.2 Tailwind 与 CSS Layer

当前 `globals.css` 已把 Tailwind Theme 和 Utilities 放入 CSS Layer。Mantine 应只引入 Layer 版本：

```ts
import "@mantine/core/styles.layer.css";
import "@/shared/styles/globals.css";
```

禁止同时导入：

```ts
import "@mantine/core/styles.css";
import "@mantine/core/styles.layer.css";
```

建议明确层顺序：

```css
@layer mantine, theme, components, utilities;
```

目标优先级：

```text
Mantine defaults
  < CSGClaw component styles/tokens
  < 有意使用的 Tailwind utilities
```

实际接入时必须用一个最小页面验证 Layer 顺序，不能只依赖导入顺序推断。

### 5.3 Color Scheme

当前主题通过 `html[data-theme="light"]` 和默认 Dark Token 工作。接入 Mantine 时必须保证：

- 现有主题设置仍是用户偏好的唯一真相。
- `MantineProvider` 的 Color Scheme 与 `data-theme` 同步。
- 切换主题不会出现一帧 Mantine 默认主题闪烁。
- Portal 内组件也能获得正确主题变量。

迁移完成前不要同时维护两套互相独立的 Theme Store。

## 6. 重点组件迁移设计

### 6.1 Button：第一批迁移

当前 Contract：

- 12 个 Variant。
- 5 个 Size。
- `active`、`iconOnly`、`loading`、`loadingLabel`。
- `IconButton` 强制要求可访问性 Label。

迁移原则：

- 保留现有 `ButtonProps` 和默认值。
- 用 Mantine Button/ActionIcon 实现内部行为。
- CSG Variant 映射由 `variantColorResolver` 或组件级 `vars` 完成。
- 不把 Mantine 默认 `variant` 暴露给业务调用方。
- Loading 时的 Accessible Name、Disabled 和 DOM 结构必须回归测试。

### 6.2 TextInput：低风险迁移

保留现有：

- className 和原生 Input Props。
- Error、Disabled、Focus Ring 和 Size 语义。
- React Hook Form 的 `ref` 和事件行为。

不因采用 Mantine 而迁移到 `@mantine/form`。

### 6.3 Tooltip：中低风险，但需验证 Trigger

当前 Tooltip 使用 Radix Provider 和 `asChild`，并通过 `data-csg-tooltip-trigger` 参与 Dialog 的 Focus 逻辑。

迁移必须验证：

- Child Ref 能否正确转发。
- Disabled Button 的 Tooltip 行为。
- Delay、Side、Offset 和 Collision。
- Portal Container。
- Dialog 首焦点不会落到 Tooltip Trigger。

因此 Tooltip 虽可早期迁移，但不能只做视觉对比。

### 6.4 Select：能力匹配高，Adapter 复杂度中等

Mantine Select 已支持：

- `searchable`
- `renderOption`
- `nothingFoundMessage`
- 泛型 Primitive Value
- Controlled Search
- Scroll Area

但 CSGClaw 必须保留：

- `onValueChange(string)` 语义。
- 空字符串选项与无选择状态的区别。
- `SelectOption.description`。
- `selectedLabel`。
- `portalContainer`。
- 现有 Size 和 Trigger/Content Class Name。

Mantine 使用 `value | null` 表达空状态；当前 CSGClaw 使用内部 Sentinel 支持 `""`。迁移时必须建立显式转换层，不能让 `""`、`null`、`undefined` 在业务表单中互相漂移。

### 6.5 Dialog/Popover/Menu：后期迁移

当前 Dialog 不只是样式封装，还包含：

- Compound Component API。
- `DialogClose asChild`。
- 自定义 Portal Container。
- Overlay Class。
- 首焦点特殊处理。
- 嵌套 Dialog 和 Agent Drawer 场景。

Mantine Modal 能覆盖产品能力，但不能直接一对一替换 Radix Props。建议：

1. 先建立 CSGClaw 高层 Dialog Contract。
2. 收敛业务侧直接使用 Compound Primitive 的场景。
3. 用兼容测试锁定 Escape、Overlay Click、Focus Return、Scroll Lock、Nested Portal。
4. 再替换底层实现。

如果某个 Compound API 无法在 Mantine 上自然实现，可以让该组件继续使用 Radix；采用 Mantine 不等于必须消灭所有 Radix。

### 6.6 AppShell/Sidebar：最后迁移

Mantine AppShell、NavLink、Tabs 和 ScrollArea 可以支撑当前工作区布局，但当前前端还需要先拆除全局 Workspace Controller。

正确顺序：

```text
业务所有权移出 Workspace
  -> WorkspacePage 变成精简 AppShellRoute
  -> 稳定 Sidebar 和 Shell Contract
  -> 再评估 Mantine AppShell 内部替换
```

否则会把“架构拆分”和“UI Library 迁移”混在同一批改动里，难以定位回归。

### 6.7 继续自研的业务组件

以下组件应继续保留 CSGClaw 业务实现：

- Conversation Message Flow。
- Composer 和 Attachment。
- Agent Activity Card。
- Agent Detail 与 Profile Editor。
- Workspace File Tree 和 Preview。
- Task Board、Timeline 和 Dependency Graph。
- Hub Detail。

它们可以组合 Mantine Button、Input、Modal 等基础组件，但不应被重写成 Mantine 示例页面。

## 7. 分阶段实施计划

### 阶段 0：建立视觉和行为基线

- 记录 Button、Input、Tooltip、Select、Dialog 的 Props Contract。
- 为键盘、Focus、Portal、Escape、Disabled、Loading 和 Empty Value 补测试。
- 建立 Light/Dark 的关键页面截图基线。
- 记录当前 JS/CSS Bundle 大小。
- 建立组件迁移清单和 Owner。

### 阶段 1：接入 Provider 和 Theme Bridge

- 引入 `styles.layer.css`。
- 在 `AppProviders` 增加 MantineProvider。
- 映射颜色、字体、圆角、阴影和 Color Scheme。
- 暂不迁移业务组件。

完成标志：

- 无组件使用 Mantine 时，现有页面视觉和主题行为完全不变。
- CSS Layer 和 Portal Theme 已验证。

### 阶段 2：迁移低风险基础组件

建议顺序：

1. Button。
2. IconButton/ActionIcon。
3. TextInput。
4. Tooltip。

每迁移一个组件：

- 保持 CSGClaw Public API。
- 运行组件测试和前端完整检查。
- 对比 Light/Dark、Hover、Focus、Disabled 和 Loading。
- 独立提交，允许单组件回滚。

### 阶段 3：迁移选择和浮层组件

建议顺序：

1. Dropdown/Menu。
2. Popover。
3. Select/Combobox。

重点验证：

- Controlled/Uncontrolled。
- Search 和 Empty Value。
- Placement/Collision。
- Portal Container 和 Z-Index。
- Nested Overlay。

### 阶段 4：迁移 Dialog

- 先收敛业务侧 Compound API。
- 建立 Focus/Keyboard/Scroll Lock 回归测试。
- 迁移普通确认框。
- 再迁移 Nested Dialog、Agent Drawer 和 Floating Chat。
- 无法自然兼容的场景继续保留 Radix。

### 阶段 5：迁移 Shell 和布局

前置条件：

- `WorkspaceControllerContext` 已拆除或进入最后删除阶段。
- `AppShellRoute` 的职责稳定。
- Sidebar 已按 Section 拆分。

然后再评估：

- AppShell。
- NavLink。
- Tabs。
- ScrollArea。
- Drawer。

### 阶段 6：清理与决策

- 删除不再使用的本地 CSS 和 Adapter。
- 检查是否还有 `radix-ui` 使用。
- 只有所有必要交互都完成迁移后，才删除 Radix 依赖。
- 更新 UI 开发规范和组件示例。
- 固化 Mantine 升级策略。

## 8. 测试与验收

### 8.1 每个组件的验收维度

- Public Props 和默认值兼容。
- DOM 事件、Ref 和 Form 行为兼容。
- Keyboard 和 Focus 行为兼容。
- ARIA Name/Description 正确。
- Light/Dark 视觉一致。
- Portal Target 和 Z-Index 正确。
- Disabled/Loading/Error/Empty 状态正确。
- 不增加业务层对 Mantine 的直接依赖。

### 8.2 工程验收

```bash
pnpm --dir web/app check
pnpm --dir web/app build
```

另外应增加：

- 关键组件 Interaction Test。
- Light/Dark Visual Regression。
- Bundle Size 对比。
- 业务关键路径人工检查：Conversation、Agent Profile、Tasks、Hub、Settings。

### 8.3 最终架构验收

- Page 和 Business Component 默认只导入 `components/ui`。
- CSGClaw Token 仍决定最终品牌视觉。
- Mantine 默认视觉没有泄漏到局部页面。
- CSS Layer 顺序明确且有测试页面验证。
- Mantine/Radix 的并存范围有清单和退出条件。
- Workspace 架构拆分与 AppShell 迁移没有在同一个大改动中执行。

## 9. 风险与应对

| 风险                        | 应对                                               |
| --------------------------- | -------------------------------------------------- |
| Mantine 默认视觉污染        | Theme Bridge + Local Wrapper + Visual Regression   |
| Tailwind/Mantine 优先级冲突 | 只使用 `styles.layer.css`，明确 Layer 顺序         |
| Props 语义变化              | Contract Test，不把 Mantine Props 直接暴露给业务层 |
| Portal/Z-Index 回归         | 统一 Portal Policy，覆盖 Nested Overlay            |
| Focus/Keyboard 回归         | Interaction Test，Dialog/Tooltip 后期迁移          |
| Bundle 增长                 | 记录迁移前后 JS/CSS，避免无关 Mantine Package      |
| 两套 UI 长期并存            | 组件清单、Owner、迁移状态和退出条件                |
| 与 Workspace 重构冲突       | 基础组件优先，AppShell 最后                        |

## 10. 不建议做的事情

- 不一次迁移全部组件。
- 不允许 Page 任意混用 Mantine 与 CSGClaw Wrapper。
- 不为采用 Mantine 而重写业务组件。
- 不同时更换 UI Library、Token、图标库和表单库。
- 不把 Mantine Theme 变成第二套独立产品设计规范。

最终判断：Mantine 值得采用，但成功标准不是“代码里出现了多少 Mantine 组件”，而是 CSGClaw 在不丢失品牌、交互和本地 API 稳定性的前提下，减少自维护基础组件的成本。
