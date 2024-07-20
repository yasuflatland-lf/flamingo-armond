package db

import (
	"time"
)

type User struct {
	ID         int64       `gorm:"column:id;primaryKey"`
	Name       string      `gorm:"column:name;not null"`
	Created    time.Time   `gorm:"column:created;autoCreateTime"`
	Updated    time.Time   `gorm:"column:updated;autoCreateTime"`
	CardGroups []Cardgroup `gorm:"many2many:cardgroup_users"`
	Roles      []Role      `gorm:"many2many:user_roles"`
}

type Card struct {
	ID           int64     `gorm:"column:id;primaryKey"`
	Front        string    `gorm:"column:front;not null"`
	Back         string    `gorm:"column:back;not null"`
	ReviewDate   time.Time `gorm:"column:review_date;not null"`
	IntervalDays int       `gorm:"column:interval_days;default:1;not null"`
	Created      time.Time `gorm:"column:created;autoCreateTime"`
	Updated      time.Time `gorm:"column:updated;autoCreateTime"`
	CardGroupID  int64     `gorm:"column:cardgroup_id;foreignKey:cardgroup_id"`
}

type Cardgroup struct {
	ID      int64     `gorm:"column:id;primaryKey"`
	Name    string    `gorm:"column:name;not null"`
	Created time.Time `gorm:"column:created;autoCreateTime"`
	Updated time.Time `gorm:"column:updated;autoCreateTime"`
	Cards   []Card    `gorm:"foreignKey:cardgroup_id"`
	Users   []User    `gorm:"many2many:cardgroup_users"`
}

type Role struct {
	ID      int64     `gorm:"column:id;primaryKey"`
	Name    string    `gorm:"column:name;not null"`
	Users   []User    `gorm:"many2many:user_roles"`
	Created time.Time `gorm:"column:created;autoCreateTime"`
	Updated time.Time `gorm:"column:updated;autoCreateTime"`
}
