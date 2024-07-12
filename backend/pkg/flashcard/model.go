package flashcard

import (
	"time"
)

type Card struct {
	ID           int64      `json:"id" gorm:"primaryKey;autoIncrement"`
	Front        string     `json:"front" gorm:"not null"`
	Back         string     `json:"back" gorm:"not null"`
	ReviewDate   time.Time  `json:"review_date" gorm:"not null"`
	IntervalDays int        `json:"interval_days" gorm:"not null;default:1"`
	Created      time.Time  `json:"created" gorm:"not null;default:CURRENT_TIMESTAMP"`
	Updated      time.Time  `json:"updated" gorm:"not null;default:CURRENT_TIMESTAMP"`
	CardGroupID  int64      `json:"cardgroup_id" gorm:"not null"`
	CardGroup    *CardGroup `json:"cardGroup" gorm:"foreignKey:CardGroupID"`
}

type CardGroup struct {
	ID      int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	Name    string    `json:"name" gorm:"not null"`
	Created time.Time `json:"created" gorm:"not null;default:CURRENT_TIMESTAMP"`
	Updated time.Time `json:"updated" gorm:"not null;default:CURRENT_TIMESTAMP"`
	Cards   []*Card   `json:"cards" gorm:"foreignKey:CardGroupID"`
	Users   []*User   `json:"users" gorm:"many2many:cardgroups_users;"`
}

type Role struct {
	ID    int64   `json:"id" gorm:"primaryKey;autoIncrement"`
	Name  string  `json:"name" gorm:"not null;unique"`
	Users []*User `json:"users" gorm:"many2many:users_roles;"`
}

type User struct {
	ID         int64        `json:"id" gorm:"primaryKey;autoIncrement"`
	Name       string       `json:"name" gorm:"not null"`
	Created    time.Time    `json:"created" gorm:"not null;default:CURRENT_TIMESTAMP"`
	Updated    time.Time    `json:"updated" gorm:"not null;default:CURRENT_TIMESTAMP"`
	CardGroups []*CardGroup `json:"cardGroups" gorm:"many2many:cardgroups_users;"`
	Roles      []*Role      `json:"roles" gorm:"many2many:users_roles;"`
}
