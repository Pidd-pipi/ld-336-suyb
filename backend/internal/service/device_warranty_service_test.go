package service

import (
	"testing"
	"time"

	"github.com/medasset/medasset/internal/constants"
	"github.com/medasset/medasset/internal/dto"
	"github.com/medasset/medasset/internal/model"
	"github.com/medasset/medasset/internal/repository"
)

// TestClassifyWarrantyBoundaries 校验剩余天数计算与分类边界：
// -1 天=已过保、0 天=今日到期（30 天内）、30 天=窗口上沿（30 天内）、31 天=不在预警范围。
func TestClassifyWarrantyBoundaries(t *testing.T) {
	now := time.Date(2026, 9, 21, 9, 0, 0, 0, time.Local)
	cases := []struct {
		offset   int
		wantType string
		wantDays int
	}{
		{-1, constants.WarrantyAlertExpired, -1},
		{-100, constants.WarrantyAlertExpired, -100},
		{0, constants.WarrantyAlertDue, 0},
		{1, constants.WarrantyAlertDue, 1},
		{30, constants.WarrantyAlertDue, 30},
		{31, "", 31},
	}
	for _, c := range cases {
		exp := now.AddDate(0, 0, c.offset)
		gotType, gotDays := classifyWarranty(&exp, now)
		if gotType != c.wantType || gotDays != c.wantDays {
			t.Errorf("offset=%d: got (%s,%d), want (%s,%d)", c.offset, gotType, gotDays, c.wantType, c.wantDays)
		}
	}

	// 到期时间带时分秒时仍按自然日计算。
	expiry := time.Date(2026, 9, 21, 23, 59, 0, 0, time.Local)
	gotType, gotDays := classifyWarranty(&expiry, time.Date(2026, 9, 21, 0, 1, 0, 0, time.Local))
	if gotType != constants.WarrantyAlertDue || gotDays != 0 {
		t.Errorf("same-day different clock: got (%s,%d), want (warranty_due,0)", gotType, gotDays)
	}

	// 无保修到期日不分类。
	if typ, _ := classifyWarranty(nil, now); typ != "" {
		t.Errorf("nil expiry should not be classified, got %s", typ)
	}
}

// TestDeviceServiceWarrantyAlerts 端到端校验预警清单：排除已报废/已禁用，类型与科室筛选生效，
// 回读字段包含责任人与剩余天数。
func TestDeviceServiceWarrantyAlerts(t *testing.T) {
	env := newTestServiceEnv(t)
	deviceRepo := repository.NewDeviceRepository(env.db)
	svc := NewDeviceService(deviceRepo, env.audit, env.logger)

	create := func(code string, offset int, status, dept, person string) {
		e := time.Now().AddDate(0, 0, offset)
		d := &model.Device{
			AssetCode: code, Name: code, Status: status, Department: dept,
			Category: "生命支持", ResponsiblePerson: person, WarrantyExpiry: &e,
		}
		if err := deviceRepo.Create(d); err != nil {
			t.Fatalf("create %s: %v", code, err)
		}
	}
	create("EXP1", -3, "in_use", "心内科", "张医生")
	create("DUE1", 12, "in_storage", "ICU", "李护士")
	create("DUE2", 30, "in_use", "心内科", "王医生")
	create("SCR1", -3, "scrapped", "心内科", "张医生")
	create("DIS1", -3, "disabled", "心内科", "张医生")

	all, err := svc.WarrantyAlerts(&dto.WarrantyAlertQuery{})
	if err != nil {
		t.Fatalf("WarrantyAlerts failed: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("len = %d, want 3", len(all))
	}
	byCode := map[string]dto.WarrantyAlertItem{}
	for _, it := range all {
		byCode[it.AssetCode] = it
	}
	if byCode["EXP1"].WarrantyType != constants.WarrantyAlertExpired || byCode["EXP1"].WarrantyDaysLeft != -3 {
		t.Errorf("EXP1 = (%s,%d)", byCode["EXP1"].WarrantyType, byCode["EXP1"].WarrantyDaysLeft)
	}
	if byCode["EXP1"].ResponsiblePerson != "张医生" {
		t.Errorf("responsible person read-back = %q", byCode["EXP1"].ResponsiblePerson)
	}
	if byCode["DUE1"].WarrantyType != constants.WarrantyAlertDue || byCode["DUE1"].WarrantyDaysLeft != 12 {
		t.Errorf("DUE1 = (%s,%d)", byCode["DUE1"].WarrantyType, byCode["DUE1"].WarrantyDaysLeft)
	}
	if byCode["DUE2"].WarrantyType != constants.WarrantyAlertDue || byCode["DUE2"].WarrantyDaysLeft != 30 {
		t.Errorf("DUE2 boundary = (%s,%d)", byCode["DUE2"].WarrantyType, byCode["DUE2"].WarrantyDaysLeft)
	}

	// 仅看过保。
	expiredOnly, err := svc.WarrantyAlerts(&dto.WarrantyAlertQuery{WarrantyType: constants.WarrantyAlertExpired})
	if err != nil || len(expiredOnly) != 1 || expiredOnly[0].AssetCode != "EXP1" {
		t.Errorf("expired filter: len=%d err=%v", len(expiredOnly), err)
	}

	// 仅看 30 天内到期。
	dueOnly, err := svc.WarrantyAlerts(&dto.WarrantyAlertQuery{WarrantyType: constants.WarrantyAlertDue})
	if err != nil || len(dueOnly) != 2 {
		t.Errorf("due filter: len=%d err=%v", len(dueOnly), err)
	}

	// 科室筛选。
	deptOnly, err := svc.WarrantyAlerts(&dto.WarrantyAlertQuery{Department: "ICU"})
	if err != nil || len(deptOnly) != 1 || deptOnly[0].AssetCode != "DUE1" {
		t.Errorf("department filter: len=%d err=%v", len(deptOnly), err)
	}
}
