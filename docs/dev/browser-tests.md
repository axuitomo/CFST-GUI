# 浏览器回归测试

前端测试分为三层。`pnpm test` 保持 Node 单测；Vitest 5 Browser Mode 在真实 Chromium 中挂载 Vapor 组件；Playwright 检查完整 WebUI 的 DOM、主题、Aria 树与布局几何。

## 安装和运行

在仓库根目录使用 PowerShell 7：

```powershell
pnpm install --frozen-lockfile
# 首次开发或缺少 frontend/bindings 时执行
wails3 generate bindings
pnpm exec playwright install chromium
pnpm test
pnpm test:browser
pnpm test:e2e
# 与字体无关的布局回归(桌面/手机 × 明暗);Windows job 也会跑
pnpm test:layout
```

Linux CI 安装浏览器时使用 `pnpm exec playwright install --with-deps chromium`。Vitest 与其 Browser Mode provider 固定为相同版本，Playwright runner 与 provider 使用的浏览器包也固定版本；升级时应一起升级并审阅基线。

`pnpm test:e2e` 运行 Chromium 行为与 Aria 测试，排除 `@layout`；`pnpm test:layout` 只运行与字体无关的布局断言。需要其他浏览器时先安装，再执行：

```powershell
pnpm exec playwright install firefox webkit
pnpm exec playwright test --project=firefox --project=webkit --grep-invert @layout
```

## 覆盖范围

- `frontend/src/components/ui/sourceControls.browser.test.ts`：反馈状态的响应式替换、重复添加同类来源、输入组模式更新、禁用按钮、实际计算的 OKLab 颜色、主题往返、390px 宽度和手机布局过渡。
- `tests/ui-regression.spec.ts`:5 个主页面在桌面 1280×900 与手机 390×844 下的 Aria 快照与布局断言；系统主题往返；固定主题保存与刷新恢复；URL 收起隐藏、编辑时显示以及切换主题后保留同一个编辑器与草稿；1023/1024px 边界反复切换时只挂载一个 Shell。用例结束时检查未捕获的页面错误。
- `tests/sources-workflow.spec.ts`：输入组复制、来源校验、预览失效、旧模式兼容及节点筛选；主动延迟旧配置保存响应，验证空组的新筛选设置不会被回写覆盖。它与整页回归复用 `tests/helpers/source-backend.ts` 的有状态 API fixture。

颜色转换保留最多 7 位小数；Stylelint 仅对 `oklab()` 允许这一精度，其他数值仍限制为 4 位，避免为满足格式规则而改变原色。

Browser Mode 使用独立配置并复用 Vite 的 Vue/Tailwind 插件；浏览器测试显式使用 Vapor 运行时。Node 配置排除 `*.browser.test.ts`，避免把组件测试放进 Node 执行。未增加组件渲染库。

布局断言只覆盖每页的首屏与列出的几何关系，不能替代页面下方内容、所有展开状态或真实设备的检查。Aria 快照是语义结构回归，不是 WCAG 合规审计。API 返回值由测试 fixture 提供，不启动真实测速、Cloudflare 写入或外部源请求。

## 基线与失败证据

- Aria 基线:`tests/__aria__/*.aria.yml`,5 个页面和来源编辑器各有桌面/手机版本;使用 `deep-equal` 匹配,新增或遗漏的语义节点都会触发回归。
- 布局回归(`@layout`,见 `tests/ui-regression.spec.ts`):每个页面在桌面/手机 × 明暗四种组合下断言与字体无关的几何关系——不出现横向溢出(容差 2px)、可见控件不被压成 0 尺寸、内容区有高度、主题底色亮度方向正确,并对每页挑一个只属于它的控件断言可见。元素缺失、布局塌陷、主题变量写错都会在这里失败。
- 不再保存整页像素基线:同一份代码在本机与 runner 上会因为 Segoe UI 版本不同而渲染出不同字形,实测差异约占全图像素的 2%(手机设置页 2.3%),与真实界面改动的量级(工具栏改版 2.5%–3.4%)重合,靠容差无法区分。几何断言两端一致,本机可以直接运行与修复。
- 普通运行禁止自动更新 Aria 快照;缺少基线也会失败;视口、语言、时区和日期都由用例固定。

有意调整语义结构后,更新对应 Aria 快照并人工审阅差异:

```powershell
pnpm exec playwright test tests/ui-regression.spec.ts --project=chromium --grep-invert @layout --update-snapshots
# 随后正常运行,确认没有依赖更新模式才能通过
pnpm test:e2e
pnpm test:layout
```
CI 的 Linux `e2e` job 运行 Browser Mode 与行为/Aria 回归；Windows compatibility job 跑同一套布局断言，覆盖 Windows 侧的桌面外壳。失败时上传 `test-results/` 和 Playwright 报告；Browser Mode 的失败截图位于 `frontend/.vitest/`，也会上传。上述运行产物已忽略，Aria 基线需要入库。

本地 Playwright 会复用 `127.0.0.1:34117` 上已有的 Vite 服务。如果更改配置或依赖后出现编译错误，先检查页面错误和该服务的输出，再重启本次开发会话的服务；不要通过删掉断言掩盖空白页。
