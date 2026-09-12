# devtools/ — CDP 调试工具链

本目录存放 CFST-GUI 浏览器自动化调试（CDP）相关的自研脚本与运行时工具环境，由 2026-09-10 仓库整理从 `cfst-results/` 剥离而来。

## 结构

```text
devtools/
├── cdp/       # 自研调试脚本（提交到版本库）
│   ├── cdp_*.py      # CDP 协议调试脚本（客户端、导航、Eval、监控、代理、原语测试）
│   ├── expr_*.js     # DevTools Expression 求值脚本
│   └── viz_debug_chain.html  # 调试链可视化页
├── lib/       # 第三方运行时库（pip --target 安装的 websocket 客户端等；保留磁盘，不提交）
└── profiles/  # 浏览器调试档案（chrome-profile/，含敏感登录数据；已被 .gitignore 忽略，不提交）
```

## 使用说明

- `cdp/` 下的 Python 脚本依赖 `lib/` 中的 websocket 库：运行前将 `lib` 加入 `PYTHONPATH`，例如

  ```powershell
  $env:PYTHONPATH = "devtools\lib"
  python devtools\cdp\cdp_client.py
  ```

- `profiles/chrome-profile/` 是真实浏览器用户数据（可能含 Cookie、登录态），**严禁提交或外传**；如需重建调试环境，可删除后由调试流程重新生成。
- `lib/` 为第三方库快照，需要时可用 `pip install --target devtools\lib websocket-client` 重建。

## 维护约定

- 新增自研调试脚本放入 `cdp/` 并提交。
- 不向 `lib/`、`profiles/` 提交任何内容（已在 `.gitignore` 覆盖）。
