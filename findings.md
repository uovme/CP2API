# CP2API 合并发现记录

## 2026-05-13
- `github.com/uovme/CP2API` 克隆后是空仓库。
- `CLIProxyAPI` 是 Go 项目，核心优势包括 CLI/OAuth 多账号、OpenAI/Gemini/Claude/Codex 兼容端点、轮询负载均衡、auth 文件目录和管理 API。
- `Sub2API` 是 Go + Vue + PostgreSQL + Redis 的完整网关平台，已有面板、用户/API Key、用量记录、额度/计费、账号调度和负载相关字段。
- 初步路线：用 Sub2API 当 CP2API 基底，再补 CLIProxyAPI token/auth 文件导入兼容和文档/API。
- CP2API 现采用 Sub2API 作为基础，因此保留面板额度消耗记录、API Key 分发、用量统计、支付、分组和账号调度能力。
- 已合入 CLIProxyAPI auth/token 文件兼容导入：`codex` 映射 OpenAI OAuth，`claude` 映射 Anthropic OAuth，`gemini` 映射 Gemini OAuth，`antigravity` 映射 Antigravity OAuth。
- 导入支持单个 JSON、JSON 数组、JSON Lines、`content` 和 `contents[]`；也兼容 Sub2API 的 `{platform,type,credentials}` 账号形状。
- 面板账号导入弹窗现在可选择 `Sub2API 导出数据` 或 `CLIProxyAPI auth/token`，后者会调用 `/api/v1/admin/accounts/import/cliproxy-auth`。
- 前端依赖安装失败根因是 registry/构建脚本审批中断导致 `node_modules` 只有 `.pnpm`，之后用 `pnpm install --frozen-lockfile --registry=https://registry.npmmirror.com --config.confirmModulesPurge=false --config.dangerouslyAllowAllBuilds=true` 补齐。
- 沙盒会阻止 Vite/Vitest/esbuild 子进程和 Go 构建缓存创建；验证命令需要提权执行。
- 推送保护根因：上游 OAuth 代码包含 Google OAuth client_id/client_secret 字面量，GitHub Push Protection 会阻止包含这些字面量的提交历史进入远端。
- 解决方案：不要在源码或文档中提交 Google OAuth client_id/client_secret 示例值；Antigravity 和 Gemini CLI 内置 OAuth 均通过环境变量注入，文档只保留 `<your-...>` 占位符。
