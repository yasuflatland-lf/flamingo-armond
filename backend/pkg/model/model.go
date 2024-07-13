package model

import (
	"time"
)

type Card struct {
	ID           int64     `gorm:"primaryKey"`
	Front        string    `gorm:"not null"`
	Back         string    `gorm:"not null"`
	ReviewDate   time.Time `gorm:"not null"`
	IntervalDays int       `gorm:"default:1;not null"`
	Created      time.Time `gorm:"autoCreateTime"`
	Updated      time.Time `gorm:"autoUpdateTime"`
	CardGroupID  int64     `gorm:"not null"`
	CardGroup    CardGroup `gorm:"foreignKey:CardGroupID"`
}

type CardGroup struct {
	ID      int64     `gorm:"primaryKey"`
	Name    string    `gorm:"not null"`
	Created time.Time `gorm:"autoCreateTime"`
	Updated time.Time `gorm:"autoUpdateTime"`
	Cards   []Card    `gorm:"foreignKey:CardGroupID"`
	Users   []User    `gorm:"many2many:user_card_groups"`
}

type Role struct {
	ID    int64  `gorm:"primaryKey"`
	Name  string `gorm:"not null"`
	Users []User `gorm:"many2many:user_roles"`
}

type User struct {
	ID         int64       `gorm:"primaryKey"`
	Name       string      `gorm:"not null"`
	Created    time.Time   `gorm:"autoCreateTime"`
	Updated    time.Time   `gorm:"autoUpdateTime"`
	CardGroups []CardGroup `gorm:"many2many:user_card_groups"`
	Roles      []Role      `gorm:"many2many:user_roles"`
}
