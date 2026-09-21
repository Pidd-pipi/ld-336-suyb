package service

import (
	"testing"
	"time"

	"github.com/medasset/medasset/internal/constants"
	"github.com/medasset/medasset/internal/model"
	"github.com/medasset/medasset/internal/repository"
)

// TestDeviceServiceWarrantyAlerts 校验保修预警的剩余天数边界、分类、过滤与状态排除。
func TestDeviceServiceWarrantyAlerts(t *testing.T) {
	env := newTestServiceEnv(t)
	repo := repository.NewDeviceRepository(env.db)
	svc := NewDeviceService(repo, env.audit, env.logger)

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, now.Location())
	at := func(days int) *time.Time {
		t := today.AddDate(0, 0, days)
		return &t
	}

	devices := []model.Device{
		{AssetCode: "W-EXPIRED", Name: "已过保设备", Status: constants.DeviceStatusInUse, Department: "放射科", Category: "影像设备", WarrantyExpiry: at(-1)},
		{AssetCode: "W-TODAY", Name: "今天到期设备", Status: constants.DeviceStatusInUse, Department: "放射科", Category: "生命支持", WarrantyExpiry: at(0)},
		{AssetCode: "W-29", Name: "29天后到期设备", Status: constants.DeviceStatusInStorage, Department: "ICU", Category: "生命支持", WarrantyExpiry: at(29)},
		{AssetCode: "W-30", Name: "30天后到期设备", Status: constants.DeviceStatusUnderMaintenance, Department: "ICU", Category: "检验设备", WarrantyExpiry: at(30)},
		{AssetCode: "W-31", Name: "31天后到期设备", Status: constants.DeviceStatusInUse, Department: "心内科", Category: "其他", WarrantyExpiry: at(31)},
		{AssetCode: "W-60", Name: "60天后到期设备", Status: constants.DeviceStatusInUse, Department: "心内科", Category: "其他", WarrantyExpiry: at(60)},
		{AssetCode: "W-SCRAPPED", Name: "已报废设备", Status: constants.DeviceStatusScrapped, Department: "放射科", Category: "影像设备", WarrantyExpiry: at(-5)},
		{AssetCode: "W-DISABLED", Name: "已禁用设备", Status: constants.DeviceStatusDisabled, Department: "放射科", Category: "影像设备", WarrantyExpiry: at(-5)},
		{AssetCode: "W-NOWARRANTY", Name: "无保修到期日设备", Status: constants.DeviceStatusInUse, Department: "放射科", Category: "影像设备"},
	}
	for i := range devices {
		if err := repo.Create(&devices[i]); err != nil {
			t.Fatalf("create %s failed: %v", devices[i].AssetCode, err)
		}
	}

	// 全部预警：已过保/今天/29/30，共 4 台；31/60 天、已报废/已禁用/无保修日均不进入。
	items, err := svc.WarrantyAlerts("", "", "")
	if err != nil {
		t.Fatalf("WarrantyAlerts failed: %v", err)
	}
	got := map[string]struct {
		alertType string
		days      int
	}{}
	for _, it := range items {
		got[it.AssetCode] = struct {
			alertType string
			days      int
		}{it.AlertType, it.WarrantyDays}
	}
	if len(items) != 4 {
		t.Fatalf("expected 4 alert items, got %d: %+v", len(items), got)
	}
	want := map[string]struct {
		alertType string
		days      int
	}{
		"W-EXPIRED": {constants.WarrantyAlertExpired, -1},
		"W-TODAY":   {constants.WarrantyAlertDue, 0},
		"W-29":      {constants.WarrantyAlertDue, 29},
		"W-30":      {constants.WarrantyAlertDue, 30},
	}
	for code, w := range want {
		g, ok := got[code]
		if !ok {
			t.Errorf("expected device %s in alert list, missing", code)
			continue
		}
		if g.alertType != w.alertType || g.days != w.days {
			t.Errorf("device %s = (%s,%d), want (%s,%d)", code, g.alertType, g.days, w.alertType, w.days)
		}
	}
	for _, excluded := range []string{"W-31", "W-60", "W-SCRAPPED", "W-DISABLED", "W-NOWARRANTY"} {
		if _, ok := got[excluded]; ok {
			t.Errorf("device %s should NOT enter warranty alert list", excluded)
		}
	}

	// 预警类型过滤：仅已过保。
	expired, err := svc.WarrantyAlerts("", "", constants.WarrantyAlertExpired)
	if err != nil {
		t.Fatalf("WarrantyAlerts(expired) failed: %v", err)
	}
	if len(expired) != 1 || expired[0].AssetCode != "W-EXPIRED" || !expired[0].WarrantyExpired {
		t.Errorf("expired filter got %+v", expired)
	}

	// 预警类型过滤：仅三十天内到期。
	due, err := svc.WarrantyAlerts("", "", constants.WarrantyAlertDue)
	if err != nil {
		t.Fatalf("WarrantyAlerts(due) failed: %v", err)
	}
	if len(due) != 3 {
		var codes []string
		for _, it := range due {
			codes = append(codes, it.AssetCode)
		}
		t.Errorf("due filter expected 3 items, got %d: %v", len(due), codes)
	}

	// 科室过滤与预警类型叠加。
	deptDue, err := svc.WarrantyAlerts("ICU", "", constants.WarrantyAlertDue)
	if err != nil {
		t.Fatalf("WarrantyAlerts(dept,due) failed: %v", err)
	}
	if len(deptDue) != 2 {
		t.Errorf("ICU due expected 2 items, got %d", len(deptDue))
	}

	// 类别过滤。
	catExpired, err := svc.WarrantyAlerts("", "影像设备", constants.WarrantyAlertExpired)
	if err != nil {
		t.Fatalf("WarrantyAlerts(category,expired) failed: %v", err)
	}
	if len(catExpired) != 1 || catExpired[0].AssetCode != "W-EXPIRED" {
		t.Errorf("影像设备 expired expected only W-EXPIRED, got %+v", catExpired)
	}

	// 结果按到期日升序（最紧急在前）。
	if items[0].AssetCode != "W-EXPIRED" {
		t.Errorf("first item should be most urgent W-EXPIRED, got %s", items[0].AssetCode)
	}
}

// TestWarrantyDaysLeft 表驱动校验剩余天数的日历天口径。
func TestWarrantyDaysLeft(t *testing.T) {
	base := time.Date(2026, 9, 21, 9, 30, 0, 0, time.Local)
	cases := []struct {
		name   string
		expiry time.Time
		want   int
	}{
		{"前一天", time.Date(2026, 9, 20, 23, 59, 0, 0, time.Local), -1},
		{"当天-早些时刻", time.Date(2026, 9, 21, 0, 1, 0, 0, time.Local), 0},
		{"当天-晚些时刻", time.Date(2026, 9, 21, 23, 59, 0, 0, time.Local), 0},
		{"30天后", time.Date(2026, 10, 21, 9, 30, 0, 0, time.Local), 30},
		{"31天后", time.Date(2026, 10, 22, 9, 30, 0, 0, time.Local), 31},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			exp := tc.expiry
			if got := warrantyDaysLeft(base, &exp); got != tc.want {
				t.Errorf("warrantyDaysLeft = %d, want %d", got, tc.want)
			}
		})
	}
	if got := warrantyDaysLeft(base, nil); got != 0 {
		t.Errorf("nil expiry days = %d, want 0", got)
	}
}
