package core

import "testing"

// 0.8.10 第三批：用户反馈"有几个源不支持共 X 集展示"。
//
// 这些源其实给了集数，只是形式不统一：纯数字、浮点、以及
// "更新至80集" / "共 80 集" / "全80话" / "80集全" 这类文本。
// 原来一律 strconv.Atoi，带一个汉字就归零，卡片上就没有"共 X 集"。
func TestNativeEpisodeCountAcceptsTextForms(t *testing.T) {
	cases := []struct {
		value any
		want  int
	}{
		{nil, 0},
		{"", 0},
		{"   ", 0},
		{"80", 80},
		{80, 80},
		{80.0, 80},
		{"更新至80集", 80},
		{"更新至 80 集", 80},
		{"共 80 集", 80},
		{"全80话", 80},
		{"80集全", 80},
		{"80话 完结", 80},
		{"已完结", 0},
		{"-5", 0},
	}
	for _, item := range cases {
		if got := nativeEpisodeCount(item.value); got != item.want {
			t.Fatalf("nativeEpisodeCount(%v) = %d, want %d", item.value, got, item.want)
		}
	}
}

// 别名也要能读到：站源给 total_episodes / episode_num 时同样要显示集数。
func TestNativeNormalizeReadsEpisodeAliases(t *testing.T) {
	cases := []struct {
		drama Drama
		want  int
	}{
		{Drama{TotalEpisodes: "更新至 66 集"}, 66},
		{Drama{TotalEpisodesSnake: 66}, 66},
		{Drama{EpisodeNum: "66"}, 66},
		{Drama{EpisodeNumSnake: 66}, 66},
		{Drama{EpisodeCount: "共 66 集", TotalEpisode: 12}, 66},
	}
	for _, item := range cases {
		if got := nativeNormalize(item.drama).Episodes; got != item.want {
			t.Fatalf("nativeNormalize episodes = %d, want %d (%+v)", got, item.want, item.drama)
		}
	}
}
