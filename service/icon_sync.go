package service

// Official icon synchronization.
//
// ## The problem it solves
//
// The console renders a model's brand mark from `models.icon`, falling back to
// the vendor's icon and finally to the first letter of the model name. On a
// fresh install both fields are empty, so every row shows a letter — which
// looks like a missing feature rather than a missing value. The upstream
// metadata feed already publishes a @lobehub/icons name for most models; this
// job copies those names in on a schedule.
//
// ## Why it is a separate job from pricing
//
// They read different feeds and fail for different reasons. A pricing run that
// cannot measure the feed currency must write nothing; an icon run has no such
// hazard and should still succeed. Bundling them would let one block the other.
//
// ## What it will not do
//
// It never invents an icon. A model the feed has no icon for keeps whatever it
// has (usually nothing, which the console renders as a letter — the honest
// fallback). And unless `overwrite` is on it never replaces an icon that is
// already set, so a hand-picked mark survives every tick.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

const (
	iconSyncTimeout  = 45 * time.Second
	iconSyncMaxBytes = 16 << 20
	iconSyncAttempts = 3
)

// ErrIconSyncBusy reports that another icon synchronization is already running.
var ErrIconSyncBusy = errors.New("an icon synchronization is already running")

type upstreamIconModel struct {
	ModelName string `json:"model_name"`
	Icon      string `json:"icon"`
}

type upstreamIconVendor struct {
	Name string `json:"name"`
	Icon string `json:"icon"`
}

// vendorIconAliases maps a local vendor name onto the names the upstream feed
// uses. The two lists are maintained independently, so a site that names a
// vendor in Chinese — or drops a suffix the feed keeps — would otherwise never
// match. Keys are lowercase.
//
// Only unambiguous names belong here. "MiniMax (minimax.cn)" and
// "MiniMax (minimax.io)" deliberately have no alias: picking one for a bare
// "MiniMax" would be a guess, and a wrong brand mark is worse than a letter.
var vendorIconAliases = map[string]string{
	"通义千问":       "alibaba",
	"qwen":       "alibaba",
	"阿里云":        "alibaba",
	"智谱":         "z.ai",
	"智谱ai":       "z.ai",
	"智谱 glm":     "z.ai",
	"glm":        "z.ai",
	"moonshot":   "moonshot ai",
	"月之暗面":       "moonshot ai",
	"kimi":       "moonshot ai",
	"xai":        "xai",
	"grok":       "xai",
	"谷歌":         "google",
	"gemini":     "google",
	"火山引擎":       "volcengine ark",
	"volcengine": "volcengine ark",
	"豆包":         "volcengine ark",
	"doubao":     "volcengine ark",
	"腾讯混元":       "tencent tokenhub",
	"混元":         "tencent tokenhub",
	"hunyuan":    "tencent tokenhub",
}

// resolveVendorIcon looks an icon up by local name, then by alias, then by a
// normalized comparison that ignores spaces and punctuation.
func resolveVendorIcon(vendorIcons map[string]string, localName string) string {
	key := strings.ToLower(strings.TrimSpace(localName))
	if icon, ok := vendorIcons[key]; ok {
		return icon
	}
	if alias, ok := vendorIconAliases[key]; ok {
		if icon, ok := vendorIcons[alias]; ok {
			return icon
		}
	}
	normalized := normalizeVendorKey(key)
	for name, icon := range vendorIcons {
		if normalizeVendorKey(name) == normalized {
			return icon
		}
	}
	return ""
}

func normalizeVendorKey(name string) string {
	var builder strings.Builder
	for _, r := range strings.ToLower(name) {
		switch r {
		case ' ', '-', '_', '.', '(', ')':
			continue
		}
		builder.WriteRune(r)
	}
	return builder.String()
}

var iconSyncMu sync.Mutex

// RunIconSyncExclusive refuses to queue: a second caller returns immediately
// with ErrIconSyncBusy rather than blocking a request handler behind a slow run.
func RunIconSyncExclusive(ctx context.Context, dryRun bool) (*model.IconSyncResult, error) {
	if !iconSyncMu.TryLock() {
		return nil, ErrIconSyncBusy
	}
	defer iconSyncMu.Unlock()
	return RunIconSync(ctx, dryRun)
}

// RunIconSync performs one synchronization. With dryRun set it reports what it
// would change and writes nothing.
func RunIconSync(ctx context.Context, dryRun bool) (*model.IconSyncResult, error) {
	setting := operation_setting.GetIconSyncSetting()
	base := strings.TrimRight(setting.SourceBase, "/")

	modelIcons, vendorIcons, err := fetchUpstreamIcons(ctx, base)
	if err != nil {
		return nil, err
	}

	localModelIcons, localVendorIcons, err := model.IconTargets()
	if err != nil {
		return nil, fmt.Errorf("read current icons: %w", err)
	}

	result := &model.IconSyncResult{
		ModelsScanned:  len(localModelIcons),
		VendorsScanned: len(localVendorIcons),
	}

	updates := make([]model.IconUpdate, 0, len(localModelIcons))
	for name, current := range localModelIcons {
		official, ok := modelIcons[name]
		if !ok || official == "" {
			result.NoUpstreamIcon++
			continue
		}
		if strings.TrimSpace(current) != "" && !setting.Overwrite {
			result.AlreadySet++
			continue
		}
		if strings.TrimSpace(current) == official {
			result.AlreadySet++
			continue
		}
		updates = append(updates, model.IconUpdate{ModelName: name, Icon: official})
	}

	vendorUpdates := make(map[string]string, len(localVendorIcons))
	for name, current := range localVendorIcons {
		official := resolveVendorIcon(vendorIcons, name)
		if official == "" {
			continue
		}
		if strings.TrimSpace(current) != "" && !setting.Overwrite {
			result.AlreadySet++
			continue
		}
		if strings.TrimSpace(current) == official {
			result.AlreadySet++
			continue
		}
		vendorUpdates[name] = official
	}

	if dryRun {
		result.ModelsUpdated = len(updates)
		result.VendorsUpdated = len(vendorUpdates)
		for index, update := range updates {
			if index == 5 {
				break
			}
			result.Sample = append(result.Sample, update.ModelName)
		}
		return result, nil
	}

	if len(updates) == 0 && len(vendorUpdates) == 0 {
		return result, nil
	}

	applied, err := model.ApplyIconSync(updates, vendorUpdates, setting.Overwrite)
	if err != nil {
		return result, err
	}
	// Carry over the counts the write path observed against the locked rows;
	// they are authoritative if an administrator edited a record mid-run.
	result.ModelsUpdated = applied.ModelsUpdated
	result.VendorsUpdated = applied.VendorsUpdated
	result.AlreadySet += applied.AlreadySet
	result.Sample = applied.Sample

	logger.LogInfo(ctx, common.LogText("icon sync finished: models=%d vendors=%d already_set=%d no_upstream=%d",
		result.ModelsUpdated, result.VendorsUpdated, result.AlreadySet, result.NoUpstreamIcon))
	return result, nil
}

// fetchUpstreamIcons downloads the model and vendor catalogs and returns the
// published icon name per model and per lowercased vendor name.
func fetchUpstreamIcons(ctx context.Context, base string) (map[string]string, map[string]string, error) {
	modelsURL := base + "/api/newapi/models.json"
	vendorsURL := base + "/api/newapi/vendors.json"

	var modelsEnv, vendorsEnv struct {
		Success bool            `json:"success"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data"`
	}
	var modelsErr, vendorsErr error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		modelsErr = fetchIconFeed(ctx, modelsURL, &modelsEnv)
	}()
	go func() {
		defer wg.Done()
		vendorsErr = fetchIconFeed(ctx, vendorsURL, &vendorsEnv)
	}()
	wg.Wait()
	if modelsErr != nil {
		return nil, nil, fmt.Errorf("fetch model catalog: %w", modelsErr)
	}
	if vendorsErr != nil {
		return nil, nil, fmt.Errorf("fetch vendor catalog: %w", vendorsErr)
	}
	if !modelsEnv.Success {
		return nil, nil, fmt.Errorf("model catalog reported failure: %s", modelsEnv.Message)
	}

	var models []upstreamIconModel
	if err := common.Unmarshal(modelsEnv.Data, &models); err != nil {
		return nil, nil, fmt.Errorf("decode model catalog: %w", err)
	}
	modelIcons := make(map[string]string, len(models))
	for _, item := range models {
		name := strings.TrimSpace(item.ModelName)
		icon := strings.TrimSpace(item.Icon)
		if name == "" || icon == "" {
			continue
		}
		modelIcons[name] = icon
	}

	vendorIcons := make(map[string]string)
	if vendorsEnv.Success {
		var vendors []upstreamIconVendor
		if err := common.Unmarshal(vendorsEnv.Data, &vendors); err == nil {
			for _, item := range vendors {
				name := strings.ToLower(strings.TrimSpace(item.Name))
				icon := strings.TrimSpace(item.Icon)
				if name == "" || icon == "" {
					continue
				}
				vendorIcons[name] = icon
			}
		}
	}
	return modelIcons, vendorIcons, nil
}

// fetchIconFeed downloads one catalog with retries. The vendor catalog is
// best-effort: a missing icon for a vendor is cosmetic, so its failure is
// reported but does not stop the model pass.
func fetchIconFeed(ctx context.Context, rawURL string, out *struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}) error {
	transport := &http.Transport{
		MaxIdleConns:          4,
		IdleConnTimeout:       60 * time.Second,
		TLSHandshakeTimeout:   15 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
	}
	if common.TLSInsecureSkipVerify {
		transport.TLSClientConfig = common.InsecureTLSConfig.Clone()
	}
	client := &http.Client{Transport: transport}

	var lastErr error
	for attempt := 0; attempt < iconSyncAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(300*(1<<attempt)) * time.Millisecond):
			}
		}
		requestCtx, cancel := context.WithTimeout(ctx, iconSyncTimeout)
		req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, rawURL, nil)
		if err != nil {
			cancel()
			return err
		}
		req.Header.Set("Accept", "application/json")
		// Icons change rarely, but a sync must not adopt a stale CDN copy when
		// a new model was just published upstream.
		req.Header.Set("Cache-Control", "no-cache")

		resp, err := client.Do(req)
		if err != nil {
			cancel()
			lastErr = err
			continue
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			cancel()
			lastErr = fmt.Errorf("%s returned %s", rawURL, resp.Status)
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, iconSyncMaxBytes))
		resp.Body.Close()
		cancel()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if err := common.Unmarshal(body, out); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	return lastErr
}
