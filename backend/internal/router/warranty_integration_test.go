package router

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/medasset/medasset/internal/config"
	"github.com/medasset/medasset/internal/constants"
	"github.com/medasset/medasset/internal/model"
	"github.com/medasset/medasset/internal/repository"
	"github.com/medasset/medasset/internal/service"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func itoa(id uint) string { return strconv.FormatUint(uint64(id), 10) }

// seedAdminForTest 创建默认管理员（与 main 启动时的 SeedAdmin 行为一致）。
func seedAdminForTest(db *gorm.DB) error {
	audit := service.NewAuditService(repository.NewAuditRepository(db), slog.Default())
	userSvc := service.NewUserService(repository.NewUserRepository(db), audit, "it-secret", 24, slog.Default())
	return userSvc.SeedAdmin()
}

type apiEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type warrantyItemJSON struct {
	ID                uint   `json:"id"`
	AssetCode         string `json:"asset_code"`
	Name              string `json:"name"`
	Department        string `json:"department"`
	Category          string `json:"category"`
	ResponsiblePerson string `json:"responsible_person"`
	Status            string `json:"status"`
	WarrantyType      string `json:"warranty_type"`
	WarrantyDaysLeft  int    `json:"warranty_days_left"`
}

func newWarrantyTestEngine(t *testing.T) (*gorm.DB, http.Handler) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.Device{}, &model.PurchaseRequest{},
		&model.MaintenanceRecord{}, &model.CalibrationRecord{}, &model.TransferRequest{},
		&model.ScrapRequest{}, &model.AuditLog{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	cfg := &config.Config{ServerPort: "8080", RunMode: "test", JWTSecret: "it-secret", RateLimit: 100000}
	engine := New(Deps{DB: db, Cfg: cfg, Log: slog.Default(), RDB: nil})
	return db, engine
}

func loginAdmin(t *testing.T, h http.Handler) string {
	t.Helper()
	body := `{"username":"admin","password":"admin123"}`
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("login status=%d body=%s", w.Code, w.Body.String())
	}
	var env apiEnvelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode login: %v", err)
	}
	var data struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(env.Data, &data); err != nil {
		t.Fatalf("decode token: %v", err)
	}
	return data.Token
}

func seedWarrantyDevices(t *testing.T, db *gorm.DB) {
	t.Helper()
	now := time.Now()
	mk := func(code string, offset int, status, dept, cat, person string) {
		e := now.AddDate(0, 0, offset)
		d := &model.Device{
			AssetCode: code, Name: code, Status: status, Department: dept, Category: cat,
			ResponsiblePerson: person, WarrantyExpiry: &e,
		}
		if err := db.Create(d).Error; err != nil {
			t.Fatalf("create %s: %v", code, err)
		}
	}
	mk("EXP1", -1, "in_use", "心内科", "生命支持", "张医生")
	mk("EXP100", -100, "in_storage", "ICU", "生命支持", "李护士")
	mk("TODAY", 0, "in_use", "心内科", "影像设备", "王医生")
	mk("DUE15", 15, "in_use", "ICU", "影像设备", "赵医生")
	mk("DUE30", 30, "in_use", "心内科", "生命支持", "孙医生")
	mk("FUTURE31", 31, "in_use", "心内科", "生命支持", "孙医生")
	mk("SCRAPPED", -5, "scrapped", "心内科", "生命支持", "张医生")
	mk("DISABLED", -5, "disabled", "ICU", "生命支持", "李护士")
}

func getWarranty(t *testing.T, h http.Handler, token, query string) []warrantyItemJSON {
	t.Helper()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/devices/warranty-warning"+query, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET warranty-warning%s status=%d body=%s", query, w.Code, w.Body.String())
	}
	var env apiEnvelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	var list []warrantyItemJSON
	if err := json.Unmarshal(env.Data, &list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	return list
}

// TestWarrantyWarningHTTP 全链路：登录鉴权 → 后端按当前日期分类 → 边界/排除/筛选/回读。
func TestWarrantyWarningHTTP(t *testing.T) {
	db, engine := newWarrantyTestEngine(t)

	// 未登录访问应被拦截（401）。
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/api/v1/devices/warranty-warning", nil)
	engine.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status=%d, want 401", w.Code)
	}

	// 种子管理员并登录。
	if err := seedAdminForTest(db); err != nil {
		t.Fatalf("seed admin: %v", err)
	}
	token := loginAdmin(t, engine)
	seedWarrantyDevices(t, db)

	toMap := func(list []warrantyItemJSON) map[string]warrantyItemJSON {
		m := map[string]warrantyItemJSON{}
		for _, it := range list {
			m[it.AssetCode] = it
		}
		return m
	}

	// 全量：含两条已过保、三条 30 天内；排除未来/报废/禁用。
	all := getWarranty(t, engine, token, "")
	if len(all) != 5 {
		t.Fatalf("all len=%d, want 5", len(all))
	}
	m := toMap(all)
	for _, excluded := range []string{"FUTURE31", "SCRAPPED", "DISABLED"} {
		if _, ok := m[excluded]; ok {
			t.Errorf("%s must not be in warning list", excluded)
		}
	}
	// 边界：-1 已过保；0/30 均为 30 天内。
	if m["EXP1"].WarrantyType != constants.WarrantyAlertExpired || m["EXP1"].WarrantyDaysLeft != -1 {
		t.Errorf("EXP1 = (%s,%d)", m["EXP1"].WarrantyType, m["EXP1"].WarrantyDaysLeft)
	}
	if m["EXP100"].WarrantyDaysLeft != -100 {
		t.Errorf("EXP100 days=%d", m["EXP100"].WarrantyDaysLeft)
	}
	if m["TODAY"].WarrantyType != constants.WarrantyAlertDue || m["TODAY"].WarrantyDaysLeft != 0 {
		t.Errorf("TODAY = (%s,%d)", m["TODAY"].WarrantyType, m["TODAY"].WarrantyDaysLeft)
	}
	if m["DUE30"].WarrantyType != constants.WarrantyAlertDue || m["DUE30"].WarrantyDaysLeft != 30 {
		t.Errorf("DUE30 boundary = (%s,%d)", m["DUE30"].WarrantyType, m["DUE30"].WarrantyDaysLeft)
	}
	if m["DUE15"].WarrantyDaysLeft != 15 {
		t.Errorf("DUE15 days=%d", m["DUE15"].WarrantyDaysLeft)
	}
	// 责任人回读。
	if m["EXP1"].ResponsiblePerson != "张医生" {
		t.Errorf("responsible read-back = %q", m["EXP1"].ResponsiblePerson)
	}

	// 按预警类型筛选。
	expiredOnly := getWarranty(t, engine, token, "?warranty_type=warranty_expired")
	if len(expiredOnly) != 2 {
		t.Errorf("expired only len=%d, want 2", len(expiredOnly))
	}
	for _, it := range expiredOnly {
		if it.WarrantyType != constants.WarrantyAlertExpired {
			t.Errorf("type filter leaked: %s", it.AssetCode)
		}
	}
	dueOnly := getWarranty(t, engine, token, "?warranty_type=warranty_due")
	if len(dueOnly) != 3 {
		t.Errorf("due only len=%d, want 3", len(dueOnly))
	}

	// 按科室 + 类别组合筛选。
	dept := getWarranty(t, engine, token, "?department="+"心内科")
	if len(dept) != 3 { // EXP1 TODAY DUE30
		t.Errorf("department filter len=%d, want 3", len(dept))
	}
	cat := getWarranty(t, engine, token, "?category="+"生命支持")
	if len(cat) != 3 { // EXP1 EXP100 DUE30
		t.Errorf("category filter len=%d, want 3", len(cat))
	}
	combo := getWarranty(t, engine, token, "?department="+"心内科"+"&category="+"生命支持")
	if len(combo) != 2 { // EXP1 DUE30
		t.Errorf("combo filter len=%d, want 2", len(combo))
	}

	// 静态预警路由与参数路由共存：/devices/:id 仍正常解析。
	wID := httptest.NewRecorder()
	reqID, _ := http.NewRequest(http.MethodGet, "/api/v1/devices/"+itoa(m["EXP1"].ID), nil)
	reqID.Header.Set("Authorization", "Bearer "+token)
	engine.ServeHTTP(wID, reqID)
	if wID.Code != http.StatusOK {
		t.Fatalf("GET /devices/:id status=%d body=%s", wID.Code, wID.Body.String())
	}

	// 回读：禁用一台即将到期设备后，预警清单应即时不再包含它（已禁用不进入清单）。
	wDis := httptest.NewRecorder()
	reqDis, _ := http.NewRequest(http.MethodPost, "/api/v1/devices/"+itoa(m["DUE15"].ID)+"/disable", nil)
	reqDis.Header.Set("Authorization", "Bearer "+token)
	engine.ServeHTTP(wDis, reqDis)
	if wDis.Code != http.StatusOK {
		t.Fatalf("disable status=%d body=%s", wDis.Code, wDis.Body.String())
	}
	after := getWarranty(t, engine, token, "")
	if len(after) != 4 {
		t.Fatalf("after disable len=%d, want 4", len(after))
	}
	for _, it := range after {
		if it.AssetCode == "DUE15" {
			t.Errorf("disabled device DUE15 must disappear after refresh read-back")
		}
	}
	// 刷新回读时该设备状态确实为 disabled。
	var d model.Device
	if err := db.First(&d, m["DUE15"].ID).Error; err != nil {
		t.Fatalf("reload device: %v", err)
	}
	if d.Status != constants.DeviceStatusDisabled {
		t.Errorf("status=%s, want disabled", d.Status)
	}
}
