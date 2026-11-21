# 维护记录

本目录归档仓库整理、兼容性审计和阶段性排查记录，按年月分桶。这里的内容用于追溯当时的判断依据，不作为当前产品行为口径；当前契约以 [`docs/dev/behavior-baseline.md`](../dev/behavior-baseline.md) 和 [`docs/reference/`](../reference/README.md) 为准。

| 时间 | 记录 | 内容 |
| --- | --- | --- |
| 2026-11-21 | [重构执行记录](./2026-11/refactor-execution-2026-11-21.md) | Phase 2 前端拆解已落地提交、实测行数与下一步切入点 |
| 2026-11-21 | [重构进度审计](./2026-11/refactor-progress-audit-2026-11-21.md) | 对照 `docs/dev/refactor-plan.md` 的分阶段完成度与风险 |
| 2026-11-20 | [DNS 兼容性检查](./2026-11/dns-compat-check-2026-11-20.md) | DNS 读取、上传更新、记录更替的兼容性核对 |
| 2026-11-20 | [DNS 多平台提交审计](./2026-11/dns-platform-commit-audit-2026-11-20.md) | 多平台 DNS 提交的范围与行为审计 |
| 2026-11-20 | [DNS 平台测试流程](./2026-11/dns-platform-test-flow-2026-11-20.md) | Cloudflare / DNSHE / 阿里云 / DNSPod 功能测试流程 |
| 2026-11-20 | [record_type 约束审计](./2026-11/dns-record-type-fix-audit-2026-11-20.md) | 类型约束是否下沉到 core 的语义评估 |
| 2026-11-20 | [DNSHE 主域名与上传审计](./2026-11/dnshe-main-domain-and-upload-audit-2026-11-20.md) | DNSHE 主域名解析与上传链路排查 |
| 2026-09-10 | [仓库整理记录](./2026-09/repo-cleanup-2026-09-10.md) | 2026-09 的文档分类、工具链归位与历史结果清理 |
