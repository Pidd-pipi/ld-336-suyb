package service

import (
	"fmt"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/medasset/medasset/internal/constants"
	"github.com/medasset/medasset/internal/dto"
	"github.com/medasset/medasset/internal/model"
	"github.com/medasset/medasset/internal/repository"
	"github.com/medasset/medasset/internal/util"
	"gorm.io/gorm"
)

// DeviceService 设备台账服务。
type DeviceService struct {
	repo  *repository.DeviceRepository
	audit *AuditService
	log   *slog.Logger
}

func NewDeviceService(repo *repository.DeviceRepository, audit *AuditService, log *slog.Logger) *DeviceService {
	return &DeviceService{repo: repo, audit: audit, log: log}
}

// Create 创建设备（台账登记）。
func (s *DeviceService) Create(req *dto.CreateDeviceReq, operator string) (*model.Device, error) {
	if _, err := s.repo.FindByAssetCode(req.AssetCode); err == nil {
		return nil, util.NewAppError(http.StatusConflict, constants.MsgDuplicateAssetCode, nil)
	}
	status := req.Status
	if status == "" {
		status = constants.DeviceStatusInStorage
	}
	device := &model.Device{
		AssetCode:           req.AssetCode,
		Barcode:             req.Barcode,
		Name:                req.Name,
		Model:               req.Model,
		Manufacturer:        req.Manufacturer,
		SerialNumber:        req.SerialNumber,
		Category:            req.Category,
		Department:          req.Department,
		ResponsiblePerson:   req.ResponsiblePerson,
		Location:            req.Location,
		Supplier:            req.Supplier,
		PurchaseDate:        req.PurchaseDate,
		PurchaseAmount:      req.PurchaseAmount,
		WarrantyMonths:      req.WarrantyMonths,
		RegistrationNo:      req.RegistrationNo,
		CertificateNo:       req.CertificateNo,
		Status:              status,
		CalibrationRequired: req.CalibrationRequired,
		PurchaseRequestID:   req.PurchaseRequestID,
	}
	if device.Barcode == "" {
		device.Barcode = util.GenBarcode(device.AssetCode)
	}
	if device.WarrantyMonths > 0 && device.PurchaseDate != nil {
		expiry := device.PurchaseDate.AddDate(0, device.WarrantyMonths, 0)
		device.WarrantyExpiry = &expiry
	}
	if err := s.repo.Create(device); err != nil {
		return nil, util.NewAppError(http.StatusInternalServerError, "创建设备失败: asset_code="+req.AssetCode, err)
	}
	s.log.Info(fmt.Sprintf(constants.LogDeviceCreated, device.ID, device.AssetCode, device.Name, device.Department, device.Status))
	s.audit.Record(0, operator, "CREATE", "device", util.Uint64String(device.ID), "设备入台账: "+device.Name, operator, "")
	return device, nil
}

// List 分页检索设备。
func (s *DeviceService) List(page, pageSize int, department, category, status, keyword string) (*util.PageResult, error) {
	list, total, err := s.repo.List(page, pageSize, department, category, status, keyword)
	if err != nil {
		return nil, util.NewAppError(http.StatusInternalServerError, constants.MsgInternalError, err)
	}
	return &util.PageResult{List: list, Total: total, Page: page, PageSize: pageSize}, nil
}

// Get 查询设备详情。
func (s *DeviceService) Get(id uint) (*dto.DeviceDetail, error) {
	d, err := s.repo.FindByID(id)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, util.NewAppError(http.StatusNotFound, "设备不存在: device_id="+util.Uint64String(id), nil)
	}
	if err != nil {
		return nil, util.NewAppError(http.StatusInternalServerError, constants.MsgInternalError, err)
	}
	expired := d.WarrantyExpiry != nil && d.WarrantyExpiry.Before(time.Now())
	return &dto.DeviceDetail{Device: *d, WarrantyExpired: expired}, nil
}

// WarrantyAlerts 保修到期预警清单。按当前日期计算剩余天数并统一分类：
// 已过保(expired) 与 三十天内到期(due)；已报废、已禁用设备不进入清单。
// department/category 为设备维度过滤，alertType 为预警类型过滤（由后端分类后再筛选）。
func (s *DeviceService) WarrantyAlerts(department, category, alertType string) ([]dto.WarrantyAlertItem, error) {
	now := time.Now()
	devices, err := s.repo.ListWarrantyAlerts(department, category, now)
	if err != nil {
		return nil, util.NewAppError(http.StatusInternalServerError, constants.MsgInternalError, err)
	}
	items := make([]dto.WarrantyAlertItem, 0, len(devices))
	for _, d := range devices {
		days := warrantyDaysLeft(now, d.WarrantyExpiry)
		typ := warrantyAlertType(days)
		if alertType != "" && alertType != typ {
			continue
		}
		items = append(items, dto.WarrantyAlertItem{
			Device:          d,
			AlertType:       typ,
			WarrantyDays:    days,
			WarrantyExpired: typ == constants.WarrantyAlertExpired,
		})
	}
	return items, nil
}

// warrantyDaysLeft 以日历天口径计算保修到期日相对当前日期的剩余天数（过期为负）。
func warrantyDaysLeft(now time.Time, expiry *time.Time) int {
	if expiry == nil {
		return 0
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	end := time.Date(expiry.Year(), expiry.Month(), expiry.Day(), 0, 0, 0, 0, now.Location())
	return int(end.Sub(today).Hours() / 24)
}

// warrantyAlertType 依据剩余天数判定预警类型：<0 已过保；0~30 三十天内到期；其余不属于预警。
func warrantyAlertType(days int) string {
	if days < 0 {
		return constants.WarrantyAlertExpired
	}
	if days <= constants.WarrantyAlertDueWindow {
		return constants.WarrantyAlertDue
	}
	return ""
}

// Update 更新设备信息。
func (s *DeviceService) Update(id uint, req *dto.UpdateDeviceReq, operator string) (*model.Device, error) {
	d, err := s.repo.FindByID(id)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, util.NewAppError(http.StatusNotFound, "设备不存在: device_id="+util.Uint64String(id), nil)
	}
	if err != nil {
		return nil, util.NewAppError(http.StatusInternalServerError, constants.MsgInternalError, err)
	}
	if d.Status == constants.DeviceStatusScrapped {
		return nil, util.NewAppError(http.StatusConflict, constants.MsgDeviceInScrapped, nil)
	}
	d.Name = req.Name
	d.Model = req.Model
	d.Manufacturer = req.Manufacturer
	d.SerialNumber = req.SerialNumber
	d.Category = req.Category
	d.Department = req.Department
	d.ResponsiblePerson = req.ResponsiblePerson
	d.Location = req.Location
	d.Supplier = req.Supplier
	d.PurchaseAmount = req.PurchaseAmount
	d.WarrantyMonths = req.WarrantyMonths
	d.RegistrationNo = req.RegistrationNo
	d.CertificateNo = req.CertificateNo
	d.CalibrationRequired = req.CalibrationRequired
	if err := s.repo.Update(d); err != nil {
		return nil, util.NewAppError(http.StatusInternalServerError, "更新设备失败: device_id="+util.Uint64String(id), err)
	}
	s.log.Info(fmt.Sprintf(constants.LogDeviceUpdated, d.ID, d.Name, d.Status))
	s.audit.Record(0, operator, "UPDATE", "device", util.Uint64String(d.ID), "更新设备: "+d.Name, operator, "")
	return d, nil
}

// Disable 禁用设备。
func (s *DeviceService) Disable(id uint, operator string) (*model.Device, error) {
	d, err := s.repo.FindByID(id)
	if err != nil {
		return nil, s.notFound(err, id)
	}
	if d.Status == constants.DeviceStatusScrapped {
		return nil, util.NewAppError(http.StatusConflict, constants.MsgDeviceInScrapped, nil)
	}
	d.Status = constants.DeviceStatusDisabled
	if err := s.repo.Update(d); err != nil {
		return nil, util.NewAppError(http.StatusInternalServerError, "禁用设备失败: device_id="+util.Uint64String(id), err)
	}
	s.log.Info(fmt.Sprintf(constants.LogDeviceDisabled, d.ID, d.AssetCode, operator))
	s.audit.Record(0, operator, "DISABLE", "device", util.Uint64String(d.ID), "禁用设备: "+d.Name, operator, "")
	return d, nil
}

// Enable 启用设备。
func (s *DeviceService) Enable(id uint, operator string) (*model.Device, error) {
	d, err := s.repo.FindByID(id)
	if err != nil {
		return nil, s.notFound(err, id)
	}
	if d.Status == constants.DeviceStatusScrapped {
		return nil, util.NewAppError(http.StatusConflict, constants.MsgDeviceInScrapped, nil)
	}
	d.Status = constants.DeviceStatusInStorage
	if err := s.repo.Update(d); err != nil {
		return nil, util.NewAppError(http.StatusInternalServerError, "启用设备失败: device_id="+util.Uint64String(id), err)
	}
	s.log.Info(fmt.Sprintf(constants.LogDeviceEnabled, d.ID, d.AssetCode, operator))
	s.audit.Record(0, operator, "ENABLE", "device", util.Uint64String(d.ID), "启用设备: "+d.Name, operator, "")
	return d, nil
}

// ChangeStatusTx 在事务中变更设备状态（被验收/调拨/报废流程复用）。
func (s *DeviceService) ChangeStatusTx(tx *gorm.DB, id uint, status string) error {
	return s.repo.UpdateStatusTx(tx, id, status)
}

func (s *DeviceService) notFound(err error, id uint) error {
	if errors.Is(err, repository.ErrNotFound) {
		return util.NewAppError(http.StatusNotFound, "设备不存在: device_id="+util.Uint64String(id), nil)
	}
	return util.NewAppError(http.StatusInternalServerError, constants.MsgInternalError, err)
}
