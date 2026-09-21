package repository

import (
	"testing"
	"time"

	"github.com/medasset/medasset/internal/model"
)

// TestDeviceRepositoryListWarrantyAlerts 校验保修预警候选集：
// 已报废/已禁用/无保修到期日/超过 30 天窗口的设备不进入候选，科室与类别过滤生效。
func TestDeviceRepositoryListWarrantyAlerts(t *testing.T) {
	db := newTestDB(t)
	repo := NewDeviceRepository(db)
	now := time.Now()
	day := func(offset int) time.Time {
		d := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, time.Local)
		return d.AddDate(0, 0, offset)
	}
	mk := func(code string, offset int, status, dept, cat string) {
		e := day(offset)
		d := &model.Device{
			AssetCode: code, Name: code, Status: status,
			Department: dept, Category: cat, WarrantyExpiry: &e,
		}
		if err := repo.Create(d); err != nil {
			t.Fatalf("create %s: %v", code, err)
		}
	}
	mk("EXPIRED", -10, "in_use", "心内科", "生命支持")     // 已过保
	mk("DUE-TODAY", 0, "in_storage", "心内科", "影像设备") // 今天到期
	mk("DUE-30", 30, "in_use", "ICU", "生命支持")       // 第 30 天到期（窗口边界）
	mk("FUTURE", 31, "in_use", "心内科", "生命支持")       // 第 31 天，窗口外
	mk("SCRAPPED", -5, "scrapped", "心内科", "生命支持")   // 已报废，排除
	mk("DISABLED", -5, "disabled", "心内科", "生命支持")   // 已禁用，排除

	var noExpiry model.Device = model.Device{AssetCode: "NOEXP", Name: "NOEXP", Status: "in_use", Department: "心内科"}
	if err := repo.Create(&noExpiry); err != nil {
		t.Fatalf("create NOEXP: %v", err)
	}

	list, err := repo.ListWarrantyAlerts("", "", 30, now)
	if err != nil {
		t.Fatalf("ListWarrantyAlerts failed: %v", err)
	}
	gotCodes := map[string]bool{}
	for _, d := range list {
		gotCodes[d.AssetCode] = true
	}
	for _, want := range []string{"EXPIRED", "DUE-TODAY", "DUE-30"} {
		if !gotCodes[want] {
			t.Errorf("expected %s in alerts, list=%v", want, gotCodes)
		}
	}
	for _, excluded := range []string{"FUTURE", "SCRAPPED", "DISABLED", "NOEXP"} {
		if gotCodes[excluded] {
			t.Errorf("%s must not enter warranty alerts", excluded)
		}
	}

	// 科室过滤。
	deptList, err := repo.ListWarrantyAlerts("心内科", "", 30, now)
	if err != nil {
		t.Fatalf("filter department: %v", err)
	}
	for _, d := range deptList {
		if d.Department != "心内科" {
			t.Errorf("department filter leaked: %s", d.AssetCode)
		}
	}
	if len(deptList) != 2 { // EXPIRED、DUE-TODAY
		t.Errorf("department list len = %d, want 2", len(deptList))
	}

	// 类别过滤。
	catList, err := repo.ListWarrantyAlerts("", "生命支持", 30, now)
	if err != nil {
		t.Fatalf("filter category: %v", err)
	}
	for _, d := range catList {
		if d.Category != "生命支持" {
			t.Errorf("category filter leaked: %s", d.AssetCode)
		}
	}
	if len(catList) != 2 { // EXPIRED、DUE-30
		t.Errorf("category list len = %d, want 2", len(catList))
	}

	// 排序：保修到期日升序。
	if len(list) >= 2 && list[0].AssetCode != "EXPIRED" {
		t.Errorf("expected first row EXPIRED, got %s", list[0].AssetCode)
	}
}
