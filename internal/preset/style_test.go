package preset

import (
	"strings"
	"testing"
)

func TestDefaultStyleIsFirstAndLookupsAgree(t *testing.T) {
	all := AllStyles()
	if len(all) == 0 || DefaultStyle().ID != all[0].ID {
		t.Fatalf("默认风格应是风格表第一项")
	}
	ids := StyleIDs()
	if len(ids) != len(all) {
		t.Fatalf("StyleIDs 数量 %d 与风格表 %d 不一致", len(ids), len(all))
	}
	seen := map[string]bool{}
	for i, style := range all {
		if ids[i] != style.ID || seen[style.ID] {
			t.Errorf("风格 id 重复或顺序不一致：%s", style.ID)
		}
		seen[style.ID] = true
		got, ok := StyleByID(style.ID)
		if !ok || got.Name != style.Name {
			t.Errorf("StyleByID(%s) 未找到", style.ID)
		}
		if doc := style.GuideDoc(); !strings.HasPrefix(doc, "docs/"+style.Name) || !strings.HasSuffix(doc, "-视频风格说明书.md") {
			t.Errorf("%s 的说明书路径不符合约定：%s", style.ID, doc)
		}
	}
}

func TestAllStylesReturnsCopy(t *testing.T) {
	all := AllStyles()
	all[0].ID = "mutated"
	if DefaultStyle().ID == "mutated" {
		t.Fatal("AllStyles 必须返回副本，调用方改动不能写回风格表")
	}
}

func TestStyleByIDOrErrorListsChoices(t *testing.T) {
	if _, err := StyleByIDOrError(DefaultStyle().ID); err != nil {
		t.Fatal(err)
	}
	_, err := StyleByIDOrError("no-such-style")
	if err == nil || !strings.Contains(err.Error(), DefaultStyle().ID) {
		t.Fatalf("未知风格应报错并列出可选项，实际 %v", err)
	}
}
