package persistence

import "time"

// 表名统一加 ddd_ 前缀,避免与宿主业务表冲突。
func (userModel) TableName() string  { return "ddd_users" }
func (orderModel) TableName() string { return "ddd_orders" }

type userModel struct {
	ID        int64     `gorm:"primaryKey;autoIncrement"`
	Name      string    `gorm:"size:64;not null"`
	Email     string    `gorm:"size:254;not null;uniqueIndex"`
	Status    string    `gorm:"size:16;not null"`
	CreatedAt time.Time `gorm:"not null"`
}

type orderItemModel struct {
	ProductName string `json:"productName"`
	UnitAmount  int64  `json:"unitAmount"`
	Currency    string `json:"currency"`
	Quantity    int    `json:"quantity"`
}

type orderModel struct {
	ID          int64            `gorm:"primaryKey;autoIncrement"`
	UserID      int64            `gorm:"index;not null"`
	Items       []orderItemModel `gorm:"serializer:json;not null"`
	TotalAmount int64            `gorm:"not null"`
	Currency    string           `gorm:"size:8;not null"`
	Status      string           `gorm:"size:16;index;not null"`
	CreatedAt   time.Time        `gorm:"not null"`
}
