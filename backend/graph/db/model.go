package db

import (
	"time"
)

// Skip fields for relations
// https://qiita.com/kiki-ki/items/5f8ec3e198f2d4b19e42
type User struct {
	ID         int64       `gorm:"column:id;primaryKey" validate:"number"`
	Name       string      `gorm:"column:name;not null" validate:"required,fl_name"`
	Created    time.Time   `gorm:"column:created;autoCreateTime"`
	Updated    time.Time   `gorm:"column:updated;autoCreateTime"`
	CardGroups []Cardgroup `gorm:"many2many:cardgroup_users" validate:"-"`
	Roles      []Role      `gorm:"many2many:user_roles" validate:"-"`
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
	CardGroup    Cardgroup `gorm:"foreignKey:CardGroupID;references:ID" validate:"-"`
}

type Cardgroup struct {
	ID      int64     `gorm:"column:id;primaryKey" validate:"number"`
	Name    string    `gorm:"column:name;not null" validate:"required,fl_name,min=1"`
	Created time.Time `gorm:"column:created;autoCreateTime"`
	Updated time.Time `gorm:"column:updated;autoCreateTime"`
	Cards   []Card    `gorm:"foreignKey:CardGroupID" validate:"-"`
	Users   []User    `gorm:"many2many:cardgroup_users" validate:"-"`
}

type CardgroupUser struct {
	CardGroupID int64     `gorm:"column:cardgroup_id;primaryKey" validate:"-"`
	UserID      int64     `gorm:"column:user_id;primaryKey" validate:"number"`
	State       int       `gorm:"column:state" validate:"number"`
	Updated     time.Time `gorm:"column:updated;autoUpdateTime"`
}

type Role struct {
	ID      int64     `gorm:"column:id;primaryKey" validate:"number"`
	Name    string    `gorm:"column:name;not null" validate:"required,fl_name,min=1"`
	Users   []User    `gorm:"many2many:user_roles" validate:"-"`
	Created time.Time `gorm:"column:created;autoCreateTime"`
	Updated time.Time `gorm:"column:updated;autoCreateTime"`
}

type SwipeRecord struct {
	ID          int64     `gorm:"column:id;primaryKey" validate:"number"`
	UserID      int64     `gorm:"column:user_id" validate:"number"`
	CardID      int64     `gorm:"column:card_id" validate:"number"`
	CardGroupID int64     `gorm:"column:cardgroup_id" validate:"number"`
	Mode        int       `gorm:"column:mode;default:1;not null" validate:"gte=0"`
	Created     time.Time `gorm:"column:created;autoCreateTime"`
	Updated     time.Time `gorm:"column:updated;autoCreateTime"`
}
