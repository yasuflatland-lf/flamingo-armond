package db

import (
	"time"
)

type User struct {
	ID         int64       `gorm:"column:id;primaryKey" validate:"number"`
	Name       string      `gorm:"column:name;not null" validate:"required,fl_name"`
	Created    time.Time   `gorm:"column:created;autoCreateTime"`
	Updated    time.Time   `gorm:"column:updated;autoCreateTime"`
	CardGroups []Cardgroup `gorm:"many2many:cardgroup_users"`
	Roles      []Role      `gorm:"many2many:user_roles"`
}

type Card struct {
	ID           int64     `gorm:"column:id;primaryKey" validate:"number"`
	Front        string    `gorm:"column:front;not null" validate:"required,min=1"`
	Back         string    `gorm:"column:back;not null" validate:"required,min=1"`
	ReviewDate   time.Time `gorm:"column:review_date;not null" validate:"fl_datetime"`
	IntervalDays int       `gorm:"column:interval_days;default:1;not null" validate:"gte=1"`
	Created      time.Time `gorm:"column:created;autoCreateTime"`
	Updated      time.Time `gorm:"column:updated;autoCreateTime"`
	CardGroupID  int64     `gorm:"column:cardgroup_id" validate:"number"`
}

type Cardgroup struct {
	ID      int64     `gorm:"column:id;primaryKey" validate:"number"`
	Name    string    `gorm:"column:name;not null" validate:"required,fl_name,min=1"`
	Created time.Time `gorm:"column:created;autoCreateTime"`
	Updated time.Time `gorm:"column:updated;autoCreateTime"`
	Cards   []Card    `gorm:"foreignKey:CardGroupID"`
	Users   []User    `gorm:"many2many:cardgroup_users"`
}

type Role struct {
	ID      int64     `gorm:"column:id;primaryKey" validate:"number"`
	Name    string    `gorm:"column:name;not null" validate:"required,fl_name,min=1"`
	Users   []User    `gorm:"many2many:user_roles"`
	Created time.Time `gorm:"column:created;autoCreateTime"`
	Updated time.Time `gorm:"column:updated;autoCreateTime"`
}
