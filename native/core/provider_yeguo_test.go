package core

import "testing"

// 详情响应里 related_list 挂着别的剧（同样带 episodes），剧级对象必须锚定在
// 请求的那一部上，否则分集标题会串到别的剧。
func TestYeguoDramaObjectAnchorsOnRequestedDrama(t *testing.T) {
	root := map[string]any{
		"video_id": "3704",
		"title":    "目标剧",
		"episodes": []any{
			map[string]any{"id": "188395", "sort": float64(1), "title": "第1集"},
		},
		"related_list": []any{
			map[string]any{
				"video_id": "3429",
				"title":    "别的剧",
				"episodes": []any{
					map[string]any{"id": "186807", "sort": float64(1), "title": "如何成为鸭王"},
				},
			},
		},
	}
	meta := yeguoDramaObject(root, "3704")
	if meta == nil {
		t.Fatal("yeguoDramaObject returned nil")
	}
	if got := yeguoText(meta, "title"); got != "目标剧" {
		t.Fatalf("anchored on %q, want 目标剧", got)
	}
	rows := yeguoEpisodeRows(meta)
	if len(rows) != 1 || rows[0].id != "188395" {
		t.Fatalf("episodes = %#v, want only the requested drama's episode", rows)
	}
}

// 分集真实字段是 id（不是 episode_id），序号是 sort；缺 sort 时按数组顺序补。
func TestYeguoEpisodeRowsUseIDAndSort(t *testing.T) {
	drama := map[string]any{
		"episodes": []any{
			map[string]any{"id": "ep-2", "sort": float64(2), "title": "第二集"},
			map[string]any{"id": "ep-1", "sort": float64(1), "title": "第一集"},
			map[string]any{"id": "ep-3", "title": "第三集"},
			map[string]any{"id": "ep-2", "sort": float64(2), "title": "重复"},
			map[string]any{"id": "ep-ad", "is_adv": true, "title": "广告"},
		},
	}
	rows := yeguoEpisodeRows(drama)
	if len(rows) != 3 {
		t.Fatalf("got %d episodes, want 3 (dedup + skip ads): %#v", len(rows), rows)
	}
	wantIDs := []string{"ep-1", "ep-2", "ep-3"}
	for index, want := range wantIDs {
		if rows[index].id != want {
			t.Fatalf("episode %d = %q, want %q (rows=%#v)", index, rows[index].id, want, rows)
		}
	}
	if rows[2].index != 3 {
		t.Fatalf("episode without sort got index %d, want 3 (array position)", rows[2].index)
	}
}

// 站点把 content_access_type 分 free/coin/vip，但播放接口并不拦付费，
// 所以列表与详情都不该把剧标成 VIP（否则详情页会弹"这是一集 VIP 内容"）。
func TestYeguoCatalogDoesNotMarkPaidDramasAsVip(t *testing.T) {
	root := map[string]any{
		"items": []any{
			map[string]any{"video_id": "3704", "title": "免费剧", "content_access_type": "free"},
			map[string]any{"video_id": "3429", "title": "金币剧", "content_access_type": "coin"},
			map[string]any{"video_id": "3516", "title": "会员剧", "content_access_type": "vip"},
		},
	}
	items := yeguoDramas(root)
	if len(items) != 3 {
		t.Fatalf("got %d dramas, want 3", len(items))
	}
	for _, item := range items {
		if item.VIP == nil || *item.VIP {
			t.Fatalf("%q marked VIP, want playable: %#v", item.Title, item.VIP)
		}
	}
}
