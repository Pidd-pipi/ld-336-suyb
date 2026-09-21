package constants

// 保修到期预警类型枚举（后端统一按当前日期计算分类，前端只展示）。
const (
	WarrantyAlertExpired = "warranty_expired" // 已过保
	WarrantyAlertDue     = "warranty_due"     // 30 天内到期
)

// WarrantyDueWindowDays 保修到期预警窗口（天）：保修到期日距今不超过该值即纳入即将到期清单。
const WarrantyDueWindowDays = 30
