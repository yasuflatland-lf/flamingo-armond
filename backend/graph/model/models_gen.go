// Code generated by github.com/99designs/gqlgen, DO NOT EDIT.

package model

import (
	"time"
)

type Card struct {
	ID           int64      `json:"id"`
	Front        string     `json:"front"`
	Back         string     `json:"back"`
	ReviewDate   time.Time  `json:"review_date"`
	IntervalDays int        `json:"interval_days"`
	Created      time.Time  `json:"created"`
	Updated      time.Time  `json:"updated"`
	CardGroup    *CardGroup `json:"cardGroup"`
}

type CardGroup struct {
	ID      int64     `json:"id"`
	Name    string    `json:"name"`
	Created time.Time `json:"created"`
	Updated time.Time `json:"updated"`
	Cards   []*Card   `json:"cards"`
	Users   []*User   `json:"users"`
}

type Mutation struct {
}

type NewCard struct {
	Front        string    `json:"front"`
	Back         string    `json:"back"`
	ReviewDate   time.Time `json:"review_date"`
	IntervalDays *int      `json:"interval_days,omitempty"`
	CardgroupID  int64     `json:"cardgroup_id"`
	Created      time.Time `json:"created"`
	Updated      time.Time `json:"updated"`
}

type NewCardGroup struct {
	Name    string    `json:"name"`
	CardIds []int64   `json:"card_ids"`
	UserIds []int64   `json:"user_ids"`
	Created time.Time `json:"created"`
	Updated time.Time `json:"updated"`
}

type NewRole struct {
	Name    string    `json:"name"`
	Created time.Time `json:"created"`
	Updated time.Time `json:"updated"`
}

type NewUser struct {
	Name    string    `json:"name"`
	RoleIds []int64   `json:"role_ids"`
	Created time.Time `json:"created"`
	Updated time.Time `json:"updated"`
}

type Query struct {
}

type Role struct {
	ID      int64     `json:"id"`
	Name    string    `json:"name"`
	Created time.Time `json:"created"`
	Updated time.Time `json:"updated"`
	Users   []*User   `json:"users"`
}

type User struct {
	ID         int64        `json:"id"`
	Name       string       `json:"name"`
	Created    time.Time    `json:"created"`
	Updated    time.Time    `json:"updated"`
	CardGroups []*CardGroup `json:"cardGroups"`
	Roles      []*Role      `json:"roles"`
}
