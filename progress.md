# CP2API 合并进度

## 2026-05-13
- 创建临时源码目录：`C:\Users\aodo\tmp\cp2api-merge-20260513-230311`
- 克隆完成：
  - `CLIProxyAPI`
  - `sub2api`
  - `CP2API`
- `CP2API` 为空仓库，已将 Sub2API 基底复制到其中，保留 `.git`。
- 已创建计划文件：`task_plan.md`、`findings.md`、`progress.md`。

## 2026-05-14
- 已实现后端 `POST /api/v1/admin/accounts/import/cliproxy-auth`，支持 CLIProxyAPI auth/token 文件和 Sub2API 账号形状导入。
- 已添加后端解析/映射单元测试：`backend/internal/handler/admin/account_cliproxy_import_test.go`。
- 已给前端账号导入弹窗添加格式选择，支持从面板导入 Sub2API 导出 JSON 或 CLIProxyAPI auth/token 文件。
- 已添加前端集成测试，覆盖 CLIProxyAPI auth 文件按原始文本调用 `importCLIProxyAuth`。
- 已通过依赖安装：`pnpm install --frozen-lockfile --registry=https://registry.npmmirror.com --config.confirmModulesPurge=false --config.dangerouslyAllowAllBuilds=true`。
- 已通过单测：`pnpm exec vitest run src/__tests__/integration/data-import.spec.ts`。
- 已通过后端相关测试：`go test ./internal/handler/admin ./internal/server/routes`。
- 已通过前端构建：`pnpm run build`。
- 首次 `git push origin main` 被 GitHub Push Protection 拦截，原因是上游 Sub2API 中存在 Google OAuth client_id/client_secret 字面量。
- 已将 Antigravity 与 Gemini CLI 的内置 OAuth client_id/client_secret 改为通过环境变量注入：
  - `ANTIGRAVITY_OAUTH_CLIENT_ID`
  - `ANTIGRAVITY_OAUTH_CLIENT_SECRET`
  - `GEMINI_CLI_OAUTH_CLIENT_ID`
  - `GEMINI_CLI_OAUTH_CLIENT_SECRET`
- 已重新执行后端验证：`go test ./internal/handler/admin ./internal/server/routes ./internal/pkg/antigravity ./internal/pkg/geminicli ./internal/service ./internal/util/logredact`，通过。
- 已重新执行前端导入测试：`pnpm exec vitest run src/__tests__/integration/data-import.spec.ts`，3 个测试通过。
- 已重新执行前端构建：`pnpm run build`，通过；仅有 Vite 分包/动态导入警告。
- 已执行 Google OAuth client_id/client_secret 相关敏感字面量扫描，无匹配。
