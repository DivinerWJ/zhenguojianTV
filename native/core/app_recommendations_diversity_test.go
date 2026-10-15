package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

// 0.6.8 回归测试：排除名单必须**跨轮累积**（修复"随机性太小"）。
//
// 用"遵守 filter_ids"的桩服务器模拟真实红果：
// 服务端按排除名单跳过已给过的剧，返回下一批。
// 若客户端丢失/重置名单，早期批次会重新出现 → 本测试失败。
func TestRecommendationFilterAccumulates(t *testing.T) {
	directory := t.TempDir()

	pool := 200
	var filterCounts []int
	transport := sourceFixtureTransport(func(request *http.Request) (*http.Response, error) {
		var payload map[string]any
		_ = json.NewDecoder(request.Body).Decode(&payload)
		filterRaw, _ := payload["filter_ids"].(string)
		filtered := map[string]bool{}
		for _, part := range strings.Split(filterRaw, ",") {
			if part = strings.TrimSpace(part); part != "" {
				filtered[part] = true
			}
		}
		filterCounts = append(filterCounts, len(filtered))
		ids := []string{}
		for i := 1; i <= pool && len(ids) < 18; i++ {
			id := strconv.Itoa(i)
			if !filtered[id] {
				ids = append(ids, id)
			}
		}
		return nativeHongguoResponse(request, 18, true, "sess-1", ids...), nil
	})

	engine := reopenCatalogEngine(t, directory, transport)
	ctx := context.Background()

	seen := map[string]bool{}
	repeats := 0
	const rounds = 6
	for round := 1; round <= rounds; round++ {
		result, err := engine.nativeRecommendations(ctx, nativeInput{
			Action: "recommendations", Category: "short_play", Force: true,
		})
		if err != nil {
			t.Fatalf("第 %d 轮失败: %v", round, err)
		}
		for _, d := range result.Items {
			if seen[d.ID] {
				repeats++
			}
			seen[d.ID] = true
		}
	}

	t.Logf("各轮服务端收到的排除数: %v", filterCounts)
	t.Logf("%d 轮共 %d 条，去重 %d 条，重复 %d 条",
		rounds, rounds*18, len(seen), repeats)

	// 1) 排除数必须**逐轮增长**（累积生效）。
	for i := 1; i < len(filterCounts); i++ {
		if filterCounts[i] <= filterCounts[i-1] {
			t.Fatalf("排除名单未累积：第 %d 轮 %d 未超过第 %d 轮 %d",
				i+1, filterCounts[i], i, filterCounts[i-1])
		}
	}
	// 2) 不应出现重复（累积正常时服务端每次都给新剧）。
	if repeats != 0 {
		t.Fatalf("出现 %d 条重复，说明排除名单未能跨轮累积", repeats)
	}
	// 3) 6 轮应拿到 108 条互不相同的剧。
	if len(seen) != rounds*18 {
		t.Fatalf("去重后应 %d 条，实际 %d 条", rounds*18, len(seen))
	}
}

// 0.6.8 回归测试：排除名单达到上限后应**自动回绕**（丢弃最早的），
// 且始终不超过接口限制 540。
func TestRecommendationFilterWrapsAtCap(t *testing.T) {
	directory := t.TempDir()

	// 池子要足够大，才能触发 540 上限（需要 30 轮 × 18 = 540）。
	pool := 2000
	var maxFilterSeen int
	transport := sourceFixtureTransport(func(request *http.Request) (*http.Response, error) {
		var payload map[string]any
		_ = json.NewDecoder(request.Body).Decode(&payload)
		filterRaw, _ := payload["filter_ids"].(string)
		filtered := map[string]bool{}
		for _, part := range strings.Split(filterRaw, ",") {
			if part = strings.TrimSpace(part); part != "" {
				filtered[part] = true
			}
		}
		if len(filtered) > maxFilterSeen {
			maxFilterSeen = len(filtered)
		}
		ids := []string{}
		for i := 1; i <= pool && len(ids) < 18; i++ {
			id := strconv.Itoa(i)
			if !filtered[id] {
				ids = append(ids, id)
			}
		}
		return nativeHongguoResponse(request, 18, true, "sess-1", ids...), nil
	})

	engine := reopenCatalogEngine(t, directory, transport)
	ctx := context.Background()

	// 跑 34 轮：足以超过 540 上限。
	for round := 1; round <= 34; round++ {
		if _, err := engine.nativeRecommendations(ctx, nativeInput{
			Action: "recommendations", Category: "short_play", Force: true,
		}); err != nil {
			t.Fatalf("第 %d 轮失败: %v", round, err)
		}
	}

	t.Logf("34 轮中服务端收到的最大排除数 = %d（上限 540）", maxFilterSeen)

	// 1) 绝不能超过接口限制（超过会被服务端判为参数无效）。
	if maxFilterSeen > 540 {
		t.Fatalf("排除数 %d 超过接口上限 540", maxFilterSeen)
	}
	// 2) 必须真的涨到过上限附近，证明是"累积到满再回绕"而非提前截断。
	if maxFilterSeen < 540 {
		t.Fatalf("排除数只到 %d，未累积到上限 540（可能仍在每轮重置）", maxFilterSeen)
	}

	// 3) 回绕后仍能正常拿到内容（不会因满而返回空）。
	result, err := engine.nativeRecommendations(ctx, nativeInput{
		Action: "recommendations", Category: "short_play", Force: true,
	})
	if err != nil {
		t.Fatalf("回绕后取数失败: %v", err)
	}
	if len(result.Items) == 0 {
		t.Fatal("回绕后应仍能取到内容")
	}
	t.Logf("回绕后仍取到 %d 条", len(result.Items))
}

// 0.6.8 回归测试：排除名单**跨重启保留**（用户反馈"重启后容易刷到上次的剧"）。
func TestRecommendationFilterSurvivesRestart(t *testing.T) {
	directory := t.TempDir()

	pool := 200
	makeTransport := func() sourceFixtureTransport {
		return sourceFixtureTransport(func(request *http.Request) (*http.Response, error) {
			var payload map[string]any
			_ = json.NewDecoder(request.Body).Decode(&payload)
			filterRaw, _ := payload["filter_ids"].(string)
			filtered := map[string]bool{}
			for _, part := range strings.Split(filterRaw, ",") {
				if part = strings.TrimSpace(part); part != "" {
					filtered[part] = true
				}
			}
			ids := []string{}
			for i := 1; i <= pool && len(ids) < 18; i++ {
				id := strconv.Itoa(i)
				if !filtered[id] {
					ids = append(ids, id)
				}
			}
			return nativeHongguoResponse(request, 18, true, "sess-1", ids...), nil
		})
	}

	ctx := context.Background()

	// 第一次运行：刷两轮。
	engine := reopenCatalogEngine(t, directory, makeTransport())
	first := map[string]bool{}
	for round := 0; round < 2; round++ {
		result, err := engine.nativeRecommendations(ctx, nativeInput{
			Action: "recommendations", Category: "short_play", Force: true,
		})
		if err != nil {
			t.Fatalf("首轮第 %d 次失败: %v", round, err)
		}
		for _, d := range result.Items {
			first[d.ID] = true
		}
	}
	t.Logf("第一次运行拿到 %d 条互不相同的剧", len(first))
	engine.downloads.close()

	// 模拟"重启软件"：用同一目录重新打开引擎。
	restarted := reopenCatalogEngine(t, directory, makeTransport())
	again, err := restarted.nativeRecommendations(ctx, nativeInput{
		Action: "recommendations", Category: "short_play", Force: true,
	})
	if err != nil {
		t.Fatalf("重启后取数失败: %v", err)
	}

	dup := 0
	for _, d := range again.Items {
		if first[d.ID] {
			dup++
		}
	}
	t.Logf("重启后拿到 %d 条，其中与上次重复 %d 条", len(again.Items), dup)

	if dup != 0 {
		t.Fatalf("重启后仍刷到上次的剧 %d 条，说明排除名单未跨重启保留", dup)
	}
}

// 辅助：让 fmt 在未使用时也不报错（本文件用于诊断输出）。
var _ = fmt.Sprintf
