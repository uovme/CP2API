package admin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type CLIProxyAuthImportRequest struct {
	Content                 string         `json:"content"`
	Contents                []string       `json:"contents"`
	Name                    string         `json:"name"`
	Notes                   *string        `json:"notes"`
	GroupIDs                []int64        `json:"group_ids"`
	ProxyID                 *int64         `json:"proxy_id"`
	Concurrency             *int           `json:"concurrency"`
	Priority                *int           `json:"priority"`
	RateMultiplier          *float64       `json:"rate_multiplier"`
	LoadFactor              *int           `json:"load_factor"`
	Extra                   map[string]any `json:"extra"`
	UpdateExisting          *bool          `json:"update_existing"`
	SkipDefaultGroupBind    *bool          `json:"skip_default_group_bind"`
	ConfirmMixedChannelRisk *bool          `json:"confirm_mixed_channel_risk"`
}

type CLIProxyAuthImportResult struct {
	Total    int                         `json:"total"`
	Created  int                         `json:"created"`
	Updated  int                         `json:"updated"`
	Skipped  int                         `json:"skipped"`
	Failed   int                         `json:"failed"`
	Items    []CLIProxyAuthImportItem    `json:"items,omitempty"`
	Warnings []CLIProxyAuthImportMessage `json:"warnings,omitempty"`
	Errors   []CLIProxyAuthImportMessage `json:"errors,omitempty"`
}

type CLIProxyAuthImportItem struct {
	Index     int    `json:"index"`
	Name      string `json:"name,omitempty"`
	Action    string `json:"action"`
	AccountID int64  `json:"account_id,omitempty"`
	Message   string `json:"message,omitempty"`
}

type CLIProxyAuthImportMessage struct {
	Index   int    `json:"index"`
	Name    string `json:"name,omitempty"`
	Message string `json:"message"`
}

type cliproxyAuthImportEntry struct {
	Index int
	Value any
}

type cliproxyAuthImportAccount struct {
	Name         string
	Platform     string
	Type         string
	Credentials  map[string]any
	Extra        map[string]any
	IdentityKeys []string
}

func (h *AccountHandler) ImportCLIProxyAuth(c *gin.Context) {
	var req CLIProxyAuthImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if req.Concurrency != nil && *req.Concurrency < 0 {
		response.BadRequest(c, "concurrency must be >= 0")
		return
	}
	if req.Priority != nil && *req.Priority < 0 {
		response.BadRequest(c, "priority must be >= 0")
		return
	}
	if req.RateMultiplier != nil && *req.RateMultiplier < 0 {
		response.BadRequest(c, "rate_multiplier must be >= 0")
		return
	}
	if req.LoadFactor != nil && *req.LoadFactor > 10000 {
		response.BadRequest(c, "load_factor must be <= 10000")
		return
	}

	entries, err := parseCLIProxyAuthImportEntries(req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if len(entries) == 0 {
		response.BadRequest(c, "请输入 CLIProxyAPI auth JSON、Sub2API credentials JSON 或 JSON 数组")
		return
	}

	executeAdminIdempotentJSON(c, "admin.accounts.import_cliproxy_auth", req, service.DefaultWriteIdempotencyTTL(), func(ctx context.Context) (any, error) {
		return h.importCLIProxyAuth(ctx, req, entries)
	})
}

func (h *AccountHandler) importCLIProxyAuth(ctx context.Context, req CLIProxyAuthImportRequest, entries []cliproxyAuthImportEntry) (CLIProxyAuthImportResult, error) {
	result := CLIProxyAuthImportResult{
		Total: len(entries),
		Items: make([]CLIProxyAuthImportItem, 0, len(entries)),
	}

	existingAccounts, err := h.listAccountsFiltered(ctx, "", "", "", "", 0, "", "created_at", "desc")
	if err != nil {
		return result, err
	}
	index := buildCLIProxyAuthAccountIndex(existingAccounts)

	updateExisting := true
	if req.UpdateExisting != nil {
		updateExisting = *req.UpdateExisting
	}
	concurrency := 3
	if req.Concurrency != nil {
		concurrency = *req.Concurrency
	}
	priority := 50
	if req.Priority != nil {
		priority = *req.Priority
	}
	skipDefaultGroupBind := false
	if req.SkipDefaultGroupBind != nil {
		skipDefaultGroupBind = *req.SkipDefaultGroupBind
	}
	skipMixedChannelCheck := req.ConfirmMixedChannelRisk != nil && *req.ConfirmMixedChannelRisk
	seenIdentity := map[string]int{}

	for _, entry := range entries {
		item, normalizeErr := normalizeCLIProxyAuthImportEntry(entry)
		if normalizeErr != nil {
			result.Failed++
			result.Items = append(result.Items, CLIProxyAuthImportItem{Index: entry.Index, Action: "failed", Message: normalizeErr.Error()})
			result.Errors = append(result.Errors, CLIProxyAuthImportMessage{Index: entry.Index, Message: normalizeErr.Error()})
			continue
		}

		accountName := buildCLIProxyAuthAccountName(req.Name, item, entry.Index, len(entries))
		if duplicateIndex, ok := firstSeenCodexIdentity(seenIdentity, item.IdentityKeys); ok {
			message := fmt.Sprintf("与第 %d 条导入项重复，已跳过", duplicateIndex)
			result.Skipped++
			result.Items = append(result.Items, CLIProxyAuthImportItem{Index: entry.Index, Name: accountName, Action: "skipped", Message: message})
			result.Warnings = append(result.Warnings, CLIProxyAuthImportMessage{Index: entry.Index, Name: accountName, Message: message})
			continue
		}
		markCodexIdentitySeen(seenIdentity, item.IdentityKeys, entry.Index)

		credentials := mergeCodexImportMap(item.Credentials, nil)
		extra := mergeCodexImportMap(req.Extra, item.Extra)

		if existing := index.Find(item.IdentityKeys); existing != nil && updateExisting {
			updated, updateErr := h.adminService.UpdateAccount(ctx, existing.ID, &service.UpdateAccountInput{
				Credentials:           mergeCodexImportMap(existing.Credentials, credentials),
				Extra:                 mergeCodexImportMap(existing.Extra, extra),
				ProxyID:               req.ProxyID,
				Concurrency:           req.Concurrency,
				Priority:              req.Priority,
				RateMultiplier:        req.RateMultiplier,
				LoadFactor:            req.LoadFactor,
				GroupIDs:              optionalGroupIDs(req.GroupIDs),
				SkipMixedChannelCheck: skipMixedChannelCheck,
			})
			if updateErr != nil {
				result.Failed++
				result.Items = append(result.Items, CLIProxyAuthImportItem{Index: entry.Index, Name: accountName, Action: "failed", Message: updateErr.Error()})
				result.Errors = append(result.Errors, CLIProxyAuthImportMessage{Index: entry.Index, Name: accountName, Message: updateErr.Error()})
				continue
			}
			if h.tokenCacheInvalidator != nil && updated != nil {
				_ = h.tokenCacheInvalidator.InvalidateToken(ctx, updated)
			}
			result.Updated++
			accountID := existing.ID
			if updated != nil {
				accountID = updated.ID
				index.Add(*updated)
			}
			result.Items = append(result.Items, CLIProxyAuthImportItem{Index: entry.Index, Name: accountName, Action: "updated", AccountID: accountID})
			continue
		}

		account, createErr := h.adminService.CreateAccount(ctx, &service.CreateAccountInput{
			Name:                  accountName,
			Notes:                 req.Notes,
			Platform:              item.Platform,
			Type:                  item.Type,
			Credentials:           credentials,
			Extra:                 extra,
			ProxyID:               req.ProxyID,
			Concurrency:           concurrency,
			Priority:              priority,
			RateMultiplier:        req.RateMultiplier,
			LoadFactor:            req.LoadFactor,
			GroupIDs:              req.GroupIDs,
			SkipDefaultGroupBind:  skipDefaultGroupBind,
			SkipMixedChannelCheck: skipMixedChannelCheck,
		})
		if createErr != nil {
			result.Failed++
			result.Items = append(result.Items, CLIProxyAuthImportItem{Index: entry.Index, Name: accountName, Action: "failed", Message: createErr.Error()})
			result.Errors = append(result.Errors, CLIProxyAuthImportMessage{Index: entry.Index, Name: accountName, Message: createErr.Error()})
			continue
		}
		if account != nil {
			index.Add(*account)
		}
		result.Created++
		accountID := int64(0)
		if account != nil {
			accountID = account.ID
		}
		result.Items = append(result.Items, CLIProxyAuthImportItem{Index: entry.Index, Name: accountName, Action: "created", AccountID: accountID})
	}

	return result, nil
}

func optionalGroupIDs(groupIDs []int64) *[]int64 {
	if len(groupIDs) == 0 {
		return nil
	}
	out := append([]int64(nil), groupIDs...)
	return &out
}

func parseCLIProxyAuthImportEntries(req CLIProxyAuthImportRequest) ([]cliproxyAuthImportEntry, error) {
	contents := make([]string, 0, 1+len(req.Contents))
	if strings.TrimSpace(req.Content) != "" {
		contents = append(contents, req.Content)
	}
	for _, content := range req.Contents {
		if strings.TrimSpace(content) != "" {
			contents = append(contents, content)
		}
	}

	var entries []cliproxyAuthImportEntry
	for _, content := range contents {
		values, err := parseCLIProxyAuthImportContent(content)
		if err != nil {
			return nil, err
		}
		for _, value := range values {
			entries = append(entries, cliproxyAuthImportEntry{Index: len(entries) + 1, Value: value})
		}
	}
	return entries, nil
}

func parseCLIProxyAuthImportContent(content string) ([]any, error) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return nil, nil
	}
	if looksLikeJSON(trimmed) {
		values, err := decodeCLIProxyAuthJSONStream(trimmed)
		if err != nil {
			if strings.Contains(trimmed, "\n") {
				if lineValues, lineErr := parseCLIProxyAuthImportLines(trimmed); lineErr == nil {
					return lineValues, nil
				}
			}
			return nil, fmt.Errorf("JSON 解析失败: %w", err)
		}
		return flattenCodexImportValues(values), nil
	}
	return parseCLIProxyAuthImportLines(trimmed)
}

func parseCLIProxyAuthImportLines(content string) ([]any, error) {
	values := make([]any, 0)
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !looksLikeJSON(line) {
			return nil, fmt.Errorf("第 %d 行不是 JSON auth 文件内容", len(values)+1)
		}
		lineValues, err := decodeCLIProxyAuthJSONStream(line)
		if err != nil {
			return nil, fmt.Errorf("第 %d 行 JSON 解析失败: %w", len(values)+1, err)
		}
		values = append(values, flattenCodexImportValues(lineValues)...)
	}
	return values, nil
}

func decodeCLIProxyAuthJSONStream(content string) ([]any, error) {
	decoder := json.NewDecoder(strings.NewReader(content))
	decoder.UseNumber()
	values := make([]any, 0, 1)
	for {
		var value any
		err := decoder.Decode(&value)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	if len(values) == 0 {
		return nil, errors.New("空 JSON 内容")
	}
	return values, nil
}

func normalizeCLIProxyAuthImportEntry(entry cliproxyAuthImportEntry) (*cliproxyAuthImportAccount, error) {
	raw, ok := entry.Value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("第 %d 条不是 JSON 对象", entry.Index)
	}

	now := time.Now().UTC()
	if isSub2APIAccountShape(raw) {
		return normalizeSub2APIAccountImportEntry(entry.Index, raw, now)
	}
	return normalizeCLIProxyAuthObject(entry.Index, raw, now)
}

func normalizeSub2APIAccountImportEntry(index int, raw map[string]any, now time.Time) (*cliproxyAuthImportAccount, error) {
	platform := normalizeCLIProxyProvider(firstCLIProxyString(raw, []string{"platform"}))
	if platform == "" {
		return nil, fmt.Errorf("第 %d 条缺少 platform", index)
	}
	accountType := strings.TrimSpace(firstCLIProxyString(raw, []string{"type"}))
	if accountType == "" {
		accountType = service.AccountTypeOAuth
	}
	if accountType != service.AccountTypeOAuth && accountType != service.AccountTypeSetupToken && accountType != service.AccountTypeAPIKey {
		return nil, fmt.Errorf("第 %d 条账号类型 %q 暂不支持导入", index, accountType)
	}
	credentials, _ := cloneStringAnyMap(raw["credentials"])
	if len(credentials) == 0 {
		return nil, fmt.Errorf("第 %d 条 credentials 为空", index)
	}
	extra, _ := cloneStringAnyMap(raw["extra"])
	stampCLIProxyExtra(extra, "sub2api", now)
	name := strings.TrimSpace(firstCLIProxyString(raw, []string{"name"}))
	keys := buildCLIProxyIdentityKeys(platform, credentials)
	if len(keys) == 0 {
		keys = append(keys, fmt.Sprintf("%s:credential:%s", platform, fingerprintCredential(credentials)))
	}
	return &cliproxyAuthImportAccount{
		Name:         name,
		Platform:     platform,
		Type:         accountType,
		Credentials:  credentials,
		Extra:        extra,
		IdentityKeys: keys,
	}, nil
}

func normalizeCLIProxyAuthObject(index int, raw map[string]any, now time.Time) (*cliproxyAuthImportAccount, error) {
	provider := strings.TrimSpace(firstCLIProxyString(raw, []string{"type"}, []string{"provider"}))
	platform := normalizeCLIProxyProvider(provider)
	if platform == "" {
		return nil, fmt.Errorf("第 %d 条不支持的 CLIProxyAPI provider: %q", index, provider)
	}

	credentials := map[string]any{}
	extra := map[string]any{}
	copyKnownCLIProxyCredential(credentials, raw, "access_token", "access_token")
	copyKnownCLIProxyCredential(credentials, raw, "refresh_token", "refresh_token")
	copyKnownCLIProxyCredential(credentials, raw, "id_token", "id_token")
	copyKnownCLIProxyCredential(credentials, raw, "account_id", "chatgpt_account_id")
	copyKnownCLIProxyCredential(credentials, raw, "project_id", "project_id")
	copyKnownCLIProxyCredential(credentials, raw, "email", "email")
	copyKnownCLIProxyCredential(credentials, raw, "expired", "expires_at")
	copyKnownCLIProxyCredential(credentials, raw, "expire", "expires_at")
	copyKnownCLIProxyCredential(credentials, raw, "expires_at", "expires_at")

	if token, ok := raw["token"].(map[string]any); ok {
		copyKnownCLIProxyCredential(credentials, token, "access_token", "access_token")
		copyKnownCLIProxyCredential(credentials, token, "refresh_token", "refresh_token")
		copyKnownCLIProxyCredential(credentials, token, "token_type", "token_type")
		copyKnownCLIProxyCredential(credentials, token, "expiry", "expires_at")
		copyKnownCLIProxyCredential(credentials, token, "expires_at", "expires_at")
	}

	if platform == service.PlatformGemini {
		if _, ok := credentials["oauth_type"]; !ok {
			credentials["oauth_type"] = "code_assist"
		}
	}

	if len(credentials) == 0 || (strings.TrimSpace(stringFromAny(credentials["access_token"])) == "" && strings.TrimSpace(stringFromAny(credentials["refresh_token"])) == "") {
		return nil, fmt.Errorf("第 %d 条缺少 access_token 或 refresh_token", index)
	}

	stampCLIProxyExtra(extra, provider, now)
	disabled, _ := raw["disabled"].(bool)
	if disabled {
		extra["cliproxy_disabled"] = true
	}
	name := buildCLIProxyImportedName(platform, provider, stringFromAny(credentials["email"]), index)
	keys := buildCLIProxyIdentityKeys(platform, credentials)
	if len(keys) == 0 {
		keys = append(keys, fmt.Sprintf("%s:credential:%s", platform, fingerprintCredential(credentials)))
	}
	return &cliproxyAuthImportAccount{
		Name:         name,
		Platform:     platform,
		Type:         service.AccountTypeOAuth,
		Credentials:  credentials,
		Extra:        extra,
		IdentityKeys: keys,
	}, nil
}

func isSub2APIAccountShape(raw map[string]any) bool {
	if _, ok := raw["credentials"].(map[string]any); ok {
		return true
	}
	return false
}

func normalizeCLIProxyProvider(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "codex", "openai", "chatgpt":
		return service.PlatformOpenAI
	case "claude", "anthropic":
		return service.PlatformAnthropic
	case "gemini", "gemini-cli":
		return service.PlatformGemini
	case "antigravity":
		return service.PlatformAntigravity
	default:
		return ""
	}
}

func copyKnownCLIProxyCredential(dst map[string]any, src map[string]any, srcKey, dstKey string) {
	value := strings.TrimSpace(stringFromAny(src[srcKey]))
	if value != "" {
		dst[dstKey] = value
	}
}

func firstCLIProxyString(raw map[string]any, paths ...[]string) string {
	for _, path := range paths {
		if value := strings.TrimSpace(stringFromAny(valueAtPath(raw, path))); value != "" {
			return value
		}
	}
	return ""
}

func valueAtPath(raw map[string]any, path []string) any {
	var current any = raw
	for _, key := range path {
		m, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current = m[key]
	}
	return current
}

func stringFromAny(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case json.Number:
		return v.String()
	default:
		return ""
	}
}

func cloneStringAnyMap(value any) (map[string]any, bool) {
	raw, ok := value.(map[string]any)
	if !ok {
		return nil, false
	}
	out := make(map[string]any, len(raw))
	for key, v := range raw {
		out[key] = v
	}
	return out, true
}

func stampCLIProxyExtra(extra map[string]any, provider string, now time.Time) {
	extra["import_source"] = "cliproxy_auth"
	extra["cliproxy_provider"] = provider
	extra["imported_at"] = now.Format(time.RFC3339)
}

func buildCLIProxyImportedName(platform, provider, email string, index int) string {
	label := platform
	if strings.TrimSpace(provider) != "" {
		label = provider
	}
	if strings.TrimSpace(email) != "" {
		return fmt.Sprintf("%s %s", label, strings.TrimSpace(email))
	}
	return fmt.Sprintf("%s imported %d", label, index)
}

func buildCLIProxyAuthAccountName(base string, item *cliproxyAuthImportAccount, index, total int) string {
	base = strings.TrimSpace(base)
	if base == "" && item != nil {
		base = strings.TrimSpace(item.Name)
	}
	if base == "" && item != nil {
		base = buildCLIProxyImportedName(item.Platform, stringFromAny(item.Extra["cliproxy_provider"]), stringFromAny(item.Credentials["email"]), index)
	}
	if total > 1 {
		return fmt.Sprintf("%s #%d", base, index)
	}
	return base
}

func buildCLIProxyIdentityKeys(platform string, credentials map[string]any) []string {
	keys := make([]string, 0, 4)
	accountID := strings.TrimSpace(stringFromAny(credentials["chatgpt_account_id"]))
	if platform == service.PlatformOpenAI && accountID != "" {
		keys = append(keys, "openai:account:"+accountID)
	}
	projectID := strings.TrimSpace(stringFromAny(credentials["project_id"]))
	email := strings.ToLower(strings.TrimSpace(stringFromAny(credentials["email"])))
	if platform == service.PlatformGemini && email != "" && projectID != "" {
		keys = append(keys, "gemini:"+email+":"+projectID)
	}
	if email != "" {
		keys = append(keys, platform+":email:"+email)
	}
	refreshToken := strings.TrimSpace(stringFromAny(credentials["refresh_token"]))
	if refreshToken != "" {
		keys = append(keys, platform+":refresh:"+hashSensitiveString(refreshToken))
	}
	return keys
}

func hashSensitiveString(value string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(value)))
	return hex.EncodeToString(sum[:])
}

func fingerprintCredential(credentials map[string]any) string {
	raw, err := json.Marshal(credentials)
	if err != nil {
		return hashSensitiveString(fmt.Sprintf("%v", credentials))
	}
	return hashSensitiveString(string(raw))
}

type cliproxyAuthAccountIndex struct {
	accountsByKey map[string]service.Account
}

func buildCLIProxyAuthAccountIndex(accounts []service.Account) cliproxyAuthAccountIndex {
	index := cliproxyAuthAccountIndex{accountsByKey: map[string]service.Account{}}
	for _, account := range accounts {
		index.Add(account)
	}
	return index
}

func (i cliproxyAuthAccountIndex) Add(account service.Account) {
	keys := buildCLIProxyIdentityKeys(account.Platform, account.Credentials)
	if len(keys) == 0 {
		keys = append(keys, fmt.Sprintf("%s:credential:%s", account.Platform, fingerprintCredential(account.Credentials)))
	}
	for _, key := range keys {
		if key != "" {
			i.accountsByKey[key] = account
		}
	}
}

func (i cliproxyAuthAccountIndex) Find(keys []string) *service.Account {
	for _, key := range keys {
		if account, ok := i.accountsByKey[key]; ok {
			return &account
		}
	}
	return nil
}
