package dto

import "github.com/medasset/medasset/internal/model"

// WarrantyAlertItem 保修到期预警清单条目（预警类型与剩余天数由后端按当前日期统一计算）。
type WarrantyAlertItem struct {
	model.Device
	WarrantyType     string `json:"warranty_type"`
	WarrantyDaysLeft int    `json:"warranty_days_left"`
}

// WarrantyAlertQuery 保修到期预警筛选条件。
type WarrantyAlertQuery struct {
	Department   string
	Category     string
	WarrantyType string // warranty_expired / warranty_due；空表示全部
}
