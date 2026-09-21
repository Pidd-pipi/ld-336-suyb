package repository

import (
	"testing"
	"time"

	"github.com/medasset/medasset/internal/constants"
	"github.com/medasset/medasset/internal/model"
)

// TestDeviceRepositoryListWarrantyAlerts 校验保修预警候选集：窗口边界、状态排除与维度过滤。
func TestDeviceRepositoryListWarrantyAlerts(t *testing.T) {
	db := newTestDB(t)
	repo := NewDeviceRepository(db)

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, now.Location())
	at := func(days int) *time.Time {
		tt := today.AddDate(0, 0, days)
		return &tt
	}

	candidates := []model.Device{
		{AssetCode: "R-EXPIRED", Name: "过期", Status: constants.DeviceStatusInUse, Department: "放射科", Category: "影像设备", WarrantyExpiry: at(-10)},
		{AssetCode: "R-30", Name: "30天", Status: constants.DeviceStatusInUse, Department: "放射科", Category: "影像设备", WarrantyExpiry: at(30)},
		{AssetCode: "R-31", Name: "31天", Status: constants.DeviceStatusInUse, Department: "放射科", Category: "影像设备", WarrantyExpiry: at(31)},
		{AssetCode: "R-SCRAPPED", Name: "报废", Status: constants.DeviceStatusScrapped, Department: "放射科", Category: "影像设备", WarrantyExpiry: at(-10)},
		{AssetCode: "R-DISABLED", Name: "禁用", Status: constants.DeviceStatusDisabled, Department: "放射科", Category: "影像设备", WarrantyExpiry: at(-10)},
		{AssetCode: "R-NIL", Name: "无保修日", Status: constants.DeviceStatusInUse, Department: "放射科", Category: "影像设备"},
	}
	for i := range candidates {
		if err := repo.Create(&candidates[i]); err != nil {
			t.Fatalf("create %s failed: %v", candidates[i].AssetCode, err)
		}
	}

	list, err := repo.ListWarrantyAlerts("", "", now)
	if err != nil {
		t.Fatalf("ListWarrantyAlerts failed: %v", err)
	}
	codes := map[string]bool{}
	for _, d := range list {
		codes[d.AssetCode] = true
	}
	if len(list) != 2 || !codes["R-EXPIRED"] || !codes["R-30"] {
		t.Errorf("candidate set = %v, want only R-EXPIRED,R-30", codes)
	}
	for _, excluded := range []string{"R-31", "R-SCRAPPED", "R-DISABLED", "R-NIL"} {
		if codes[excluded] {
			t.Errorf("%s should not be a warranty alert candidate", excluded)
		}
	}

	// 维度过滤：科室 + 类别（命中）。
	filtered, err := repo.ListWarrantyAlerts("放射科", "影像设备", now)
	if err != nil || len(filtered) != 2 {
		t.Errorf("dept+category filter = %d, err=%v, want 2", len(filtered), err)
	}

	// 维度过滤：不匹配的类别。
	none, err := repo.ListWarrantyAlerts("", "其他", now)
	if err != nil || len(none) != 0 {
		t.Errorf("non-matching category filter = %d, err=%v, want 0", len(none), err)
	}

	// 排序：到期日升序（最紧急在前）。
	if len(list) == 2 && list[0].AssetCode != "R-EXPIRED" {
		t.Errorf("order[0] = %s, want R-EXPIRED", list[0].AssetCode)
	}
}
