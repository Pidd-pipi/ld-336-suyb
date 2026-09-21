package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/medasset/medasset/internal/constants"
	"github.com/medasset/medasset/internal/dto"
	"github.com/medasset/medasset/internal/middleware"
	"github.com/medasset/medasset/internal/model"
	"github.com/medasset/medasset/internal/repository"
	"github.com/medasset/medasset/internal/service"
	"github.com/medasset/medasset/internal/util"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newWarrantyTestEngine(t *testing.T) (*gin.Engine, *repository.DeviceRepository) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.Device{}, &model.PurchaseRequest{},
		&model.MaintenanceRecord{}, &model.CalibrationRecord{}, &model.TransferRequest{},
		&model.ScrapRequest{}, &model.AuditLog{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	audit := service.NewAuditService(repository.NewAuditRepository(db), slog.Default())
	repo := repository.NewDeviceRepository(db)
	deviceSvc := service.NewDeviceService(repo, audit, slog.Default())
	h := NewDeviceHandler(deviceSvc)

	r := gin.New()
	r.Use(middleware.ErrorHandler(slog.Default()))
	g := r.Group("/api/v1/devices")
	g.GET("/warranty-alerts", h.WarrantyAlerts)
	g.POST("/:id/disable", func(c *gin.Context) {
		c.Set(middleware.UserKey, &util.Claims{Username: "tester"})
		h.Disable(c)
	})
	return r, repo
}

type warrantyAlertResp struct {
	Code int                     `json:"code"`
	Data []dto.WarrantyAlertItem `json:"data"`
}

func doWarrantyAlerts(t *testing.T, r *gin.Engine, query string) (int, warrantyAlertResp) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/devices/warranty-alerts?"+query, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var out warrantyAlertResp
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal resp: %v body=%s", err, w.Body.String())
	}
	return w.Code, out
}

// TestWarrantyAlertsHTTP 端到端验证：边界分类、非法预警类型、维度过滤，以及禁用后刷新回读。
func TestWarrantyAlertsHTTP(t *testing.T) {
	r, repo := newWarrantyTestEngine(t)
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, now.Location())
	day := func(n int) *time.Time {
		v := today.AddDate(0, 0, n)
		return &v
	}

	seed := []struct {
		code, dept, cat, status string
		expiry                  *time.Time
	}{
		{"H-EXP", "放射科", "影像设备", constants.DeviceStatusInUse, day(-1)},
		{"H-DUE", "ICU", "生命支持", constants.DeviceStatusInUse, day(30)},
		{"H-FAR", "心内科", "其他", constants.DeviceStatusInUse, day(31)},
		{"H-OFF", "放射科", "影像设备", constants.DeviceStatusDisabled, day(-1)},
		{"H-SCRAP", "放射科", "影像设备", constants.DeviceStatusScrapped, day(-1)},
	}
	for _, s := range seed {
		d := &model.Device{
			AssetCode: s.code, Name: s.code, Department: s.dept, Category: s.cat,
			Status: s.status, WarrantyExpiry: s.expiry,
		}
		if err := repo.Create(d); err != nil {
			t.Fatalf("create %s: %v", s.code, err)
		}
	}

	// 全部预警：仅过期在用品 + 第30天在用品 = 2；第31天/禁用/报废均不进入。
	httpCode, all := doWarrantyAlerts(t, r, "")
	if httpCode != http.StatusOK || all.Code != 0 {
		t.Fatalf("unexpected http=%d code=%d", httpCode, all.Code)
	}
	if len(all.Data) != 2 {
		t.Fatalf("expected 2 alerts, got %d: %+v", len(all.Data), all.Data)
	}
	byCode := map[string]dto.WarrantyAlertItem{}
	for _, it := range all.Data {
		byCode[it.AssetCode] = it
	}
	if byCode["H-EXP"].AlertType != constants.WarrantyAlertExpired || byCode["H-EXP"].WarrantyDays != -1 {
		t.Errorf("H-EXP classification wrong: %+v", byCode["H-EXP"])
	}
	if byCode["H-DUE"].AlertType != constants.WarrantyAlertDue || byCode["H-DUE"].WarrantyDays != 30 {
		t.Errorf("H-DUE classification wrong: %+v", byCode["H-DUE"])
	}

	// 预警类型过滤。
	if _, got := doWarrantyAlerts(t, r, "alert_type=expired"); len(got.Data) != 1 || got.Data[0].AssetCode != "H-EXP" {
		t.Errorf("alert_type=expired got %+v", got.Data)
	}
	if _, got := doWarrantyAlerts(t, r, "alert_type=due"); len(got.Data) != 1 || got.Data[0].AssetCode != "H-DUE" {
		t.Errorf("alert_type=due got %+v", got.Data)
	}
	// 科室 + 类别 + 类型叠加（放射科/影像设备/已过保）。
	if _, got := doWarrantyAlerts(t, r, "department=%E6%94%BE%E5%B0%84%E7%A7%91&category=%E5%BD%B1%E5%83%8F%E8%AE%BE%E5%A4%87&alert_type=expired"); len(got.Data) != 1 {
		t.Errorf("combined filter got %d", len(got.Data))
	}
	// 非法预警类型应被拒绝（业务码非 0）。
	if _, got := doWarrantyAlerts(t, r, "alert_type=bogus"); got.Code == 0 {
		t.Errorf("expected non-zero code for bogus alert_type, got %+v", got)
	}

	// 刷新后回读：禁用唯一的“过期在用品” H-EXP 后，再次查询预警清单应只剩 H-DUE。
	expID := byCode["H-EXP"].ID
	req := httptest.NewRequest(http.MethodPost, "/api/v1/devices/"+strconv.Itoa(int(expID))+"/disable", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("disable status=%d body=%s", w.Code, w.Body.String())
	}
	_, after := doWarrantyAlerts(t, r, "")
	if len(after.Data) != 1 || after.Data[0].AssetCode != "H-DUE" {
		t.Errorf("after disable & refresh expected only H-DUE, got %+v", after.Data)
	}
}
