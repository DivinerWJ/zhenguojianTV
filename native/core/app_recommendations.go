package core

import (
	"context"
	"errors"
	"time"
)

type nativeRecommendationState struct {
	Query   hongguoRecommendationQuery `json:"query"`
	Page    int                        `json:"page"`
	HasMore bool                       `json:"hasMore"`
}

func nativeRecommendationKey(genre string) string { return sourceHongguo + "|recommendation:" + genre }

func (engine *nativeEngine) cachedRecommendations(genre string) (nativeCatalogResult, error) {
	if err := (hongguoRecommendationQuery{Genre: genre}).validate(); err != nil {
		return nativeCatalogResult{}, err
	}
	result := engine.nativeCached(nativeRecommendationKey(genre))
	engine.mu.Lock()
	_, found := engine.recommendations[genre]
	engine.mu.Unlock()
	if !found && len(result.Items) > 0 {
		result.Fresh, result.HasMore = false, false
		result.Warning = joinNativeWarnings(result.Warning, "推荐位置已失效，请刷新推荐")
	}
	return result, nil
}

func (engine *nativeEngine) nativeRecommendations(ctx context.Context, input nativeInput) (nativeCatalogResult, error) {
	genre := input.Category
	if err := (hongguoRecommendationQuery{Genre: genre}).validate(); err != nil {
		return nativeCatalogResult{}, err
	}
	unlock, err := engine.lockSourceCatalog(ctx, sourceHongguo)
	if err != nil {
		return nativeCatalogResult{}, err
	}
	defer unlock()
	if retried, err := engine.retryCatalogSave(); retried {
		cached, _ := engine.cachedRecommendations(genre)
		if err != nil || len(cached.Items) > 0 {
			return cached, nil
		}
	}
	cached, _ := engine.cachedRecommendations(genre)
	engine.mu.Lock()
	state, found := engine.recommendations[genre]
	engine.mu.Unlock()
	if !input.Force && found && len(cached.Items) > 0 && (input.Command != "more" || !state.HasMore) {
		return cached, nil
	}
	if !input.Force && !found && len(cached.Items) > 0 {
		return cached, nil
	}
	// 已推荐过的 ID（用于强制刷新时继续排除，避免"每次开机都是同一批"）。
	//
	// 来源有两处，合并去重：
	// 1. 上一次的内存状态 `state.Query.Seen`（本次运行内翻过的页）；
	// 2. 磁盘缓存里的条目（上次运行留下的，重启后内存状态已丢）。
	// 上限 540 与 `hongguoRecommendationQuery.validate()` 保持一致，
	// 超出会被判为参数无效。
	accumulated := make([]string, 0, 540)
	accumulatedSeen := make(map[string]bool, 540)
	appendAccumulated := func(id string) {
		if id == "" || accumulatedSeen[id] || len(accumulated) >= 540 {
			return
		}
		accumulatedSeen[id] = true
		accumulated = append(accumulated, id)
	}
	for _, id := range state.Query.Seen {
		appendAccumulated(id)
	}
	for _, drama := range cached.Items {
		appendAccumulated(drama.ID)
	}
	if input.Force || !found {
		// 0.6.5 修复（用户反馈"随机刷新不太随机，重启软件很容易出现
		// 上次启动软件后的随机内容"）：
		// 原先强制刷新会把整个 query 清空（**含 `Seen`**），于是 `filter_ids`
		// 为空、游标又是 0 → 服务端每次返回的都是**同一批第一页**，
		// 表现为"每次开机内容都差不多"。
		// 现在改为：**游标归零**（重新从池子里挑），但把**已推荐过的 ID
		// 继续交给服务端排除**，这样拿到的是新的一批。
		state = nativeRecommendationState{
			Query:   hongguoRecommendationQuery{Genre: genre, Seen: accumulated},
			HasMore: true,
		}
	}
	page, err := engine.downloader.fetchHongguoRecommendations(ctx, state.Query)
	if err != nil {
		if len(cached.Items) == 0 || errors.Is(err, context.Canceled) {
			return nativeCatalogResult{}, err
		}
		cached.Fresh = false
		cached.Warning = joinNativeWarnings(cached.Warning, publicError(err).Error())
		return cached, nil
	}
	if err := ctx.Err(); err != nil {
		return nativeCatalogResult{}, err
	}
	// 排除名单把可选池排空时（返回 0 条），清掉名单再取一次。
	// 否则用户会看到"刷新之后一条都没有"，比拿到旧内容更糟。
	if input.Force && len(page.Dramas) == 0 && len(accumulated) > 0 {
		state.Query.Seen = nil
		accumulated = nil
		page, err = engine.downloader.fetchHongguoRecommendations(ctx, state.Query)
		if err != nil {
			if len(cached.Items) == 0 || errors.Is(err, context.Canceled) {
				return nativeCatalogResult{}, err
			}
			cached.Fresh = false
			cached.Warning = joinNativeWarnings(cached.Warning, publicError(err).Error())
			return cached, nil
		}
		if err := ctx.Err(); err != nil {
			return nativeCatalogResult{}, err
		}
	}
	engine.mu.Lock()
	defer engine.mu.Unlock()
	if engine.recommendations == nil {
		engine.recommendations = map[string]nativeRecommendationState{}
	}
	key := nativeRecommendationKey(genre)
	previous := engine.catalogs[key]
	if input.Force {
		previous = nil
	}
	seen := make(map[string]bool, len(previous))
	for _, drama := range previous {
		seen[drama.ID] = true
	}
	fresh := make([]nativeDrama, 0, len(page.Dramas))
	known := make(map[string]nativeDrama, len(engine.catalogs[sourceHongguo]))
	for _, drama := range engine.catalogs[sourceHongguo] {
		known[drama.ID] = drama
	}
	for _, drama := range page.Dramas {
		item := nativeNormalize(drama)
		if old, exists := known[item.ID]; exists {
			item = mergeNativeDrama(old, item)
		}
		if !seen[item.ID] {
			seen[item.ID] = true
			fresh = append(fresh, item)
		}
	}
	if len(fresh) == 0 && page.HasMore {
		return nativeCatalogResult{}, errors.New("暂时没有新的推荐，已保留上次位置；可重试或刷新推荐")
	}
	items := mergeNativeCatalog(previous, fresh)
	identifiers := make([]string, 0, min(540, len(items)))
	for _, drama := range items[max(0, len(items)-540):] {
		identifiers = append(identifiers, drama.ID)
	}
	session := page.SessionID
	if session == "" {
		session = state.Query.SessionID
	}
	state.Query = hongguoRecommendationQuery{Genre: genre, Offset: page.NextOffset, SessionID: session, Seen: identifiers}
	state.Page++
	state.HasMore = page.HasMore
	engine.recommendations[genre] = state
	engine.catalogs[key] = items
	engine.catalogs[sourceHongguo] = mergeNativeCatalog(engine.catalogs[sourceHongguo], fresh)
	engine.hongguoCatalog = engine.downloader.hongguoCatalogSnapshot()
	engine.catalogStates[key] = nativeCatalogState{UpdatedAt: time.Now(), Page: state.Page, HasMore: state.HasMore}
	result := nativeCatalogResult{Items: append([]nativeDrama{}, items...), Page: state.Page, HasMore: state.HasMore, Fresh: true}
	if err := engine.writeCatalogDiskLocked(); err != nil {
		result.Fresh, result.Warning, result.saveError = false, err.Error(), err
	}
	return result, nil
}
