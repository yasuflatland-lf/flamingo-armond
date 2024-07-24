package db

import (
	"time"
)

type User struct {
	ID         int64       `gorm:"column:id;primaryKey"`
	Name       string      `gorm:"column:name;not null" validate:"required,fl_name"`
	Created    time.Time   `gorm:"column:created;autoCreateTime" validate:"required"`
	Updated    time.Time   `gorm:"column:updated;autoCreateTime" validate:"required"`
	CardGroups []Cardgroup `gorm:"many2many:cardgroup_users"`
	Roles      []Role      `gorm:"many2many:user_roles"`
}

type Card struct {
	ID           int64     `gorm:"column:id;primaryKey"`
	Front        string    `gorm:"column:front;not null" validate:"required"`
	Back         string    `gorm:"column:back;not null" validate:"required"`
	ReviewDate   time.Time `gorm:"column:review_date;not null" validate:"required"`
	IntervalDays int       `gorm:"column:interval_days;default:1;not null" validate:"required"`
	Created      time.Time `gorm:"column:created;autoCreateTime" validate:"required"`
	Updated      time.Time `gorm:"column:updated;autoCreateTime" validate:"required"`
	CardGroupID  int64     `gorm:"column:cardgroup_id" validate:"required"`
}

type Cardgroup struct {
	ID      int64     `gorm:"column:id;primaryKey"`
	Name    string    `gorm:"column:name;not null" validate:"required,fl_name"`
	Created time.Time `gorm:"column:created;autoCreateTime" validate:"required"`
	Updated time.Time `gorm:"column:updated;autoCreateTime" validate:"required"`
	Cards   []Card    `gorm:"foreignKey:CardGroupID"`
	Users   []User    `gorm:"many2many:cardgroup_users"`
}

type Role struct {
	ID      int64     `gorm:"column:id;primaryKey"`
	Name    string    `gorm:"column:name;not null" validate:"required,fl_name"`
	Users   []User    `gorm:"many2many:user_roles"`
	Created time.Time `gorm:"column:created;autoCreateTime" validate:"required"`
	Updated time.Time `gorm:"column:updated;autoCreateTime" validate:"required"`
}
