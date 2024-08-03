// Code generated by github.com/99designs/gqlgen, DO NOT EDIT.

package model

import (
	"time"
)

type Card struct {
	ID           int64      `json:"id"`
	Front        string     `json:"front" validate:"required,min=1"`
	Back         string     `json:"back" validate:"required,min=1"`
	ReviewDate   time.Time  `json:"review_date"`
	IntervalDays int        `json:"interval_days" validate:"gte=1"`
	Created      time.Time  `json:"created"`
	Updated      time.Time  `json:"updated"`
	CardGroupID  int64      `json:"cardGroupID"`
	CardGroup    *CardGroup `json:"cardGroup" validate:"-"`
}

type CardConnection struct {
	Edges      []*CardEdge `json:"edges,omitempty" validate:"-"`
	Nodes      []*Card     `json:"nodes,omitempty" validate:"-"`
	PageInfo   *PageInfo   `json:"pageInfo"`
	TotalCount int         `json:"totalCount"`
}

type CardEdge struct {
	Cursor int64 `json:"cursor"`
	Node   *Card `json:"node" validate:"-"`
}

type CardGroup struct {
	ID      int64           `json:"id"`
	Name    string          `json:"name" validate:"required,fl_name,min=1"`
	Created time.Time       `json:"created"`
	Updated time.Time       `json:"updated"`
	Cards   *CardConnection `json:"cards" validate:"-"`
	Users   *UserConnection `json:"users" validate:"-"`
}

type CardGroupConnection struct {
	Edges      []*CardGroupEdge `json:"edges,omitempty" validate:"-"`
	Nodes      []*CardGroup     `json:"nodes,omitempty" validate:"-"`
	PageInfo   *PageInfo        `json:"pageInfo"`
	TotalCount int              `json:"totalCount"`
}

type CardGroupEdge struct {
	Cursor int64      `json:"cursor"`
	Node   *CardGroup `json:"node" validate:"-"`
}

type Mutation struct {
}

type NewCard struct {
	Front        string    `json:"front" validate:"required,min=1"`
	Back         string    `json:"back" validate:"required,min=1"`
	ReviewDate   time.Time `json:"review_date"`
	IntervalDays *int      `json:"interval_days,omitempty" validate:"gte=1"`
	CardgroupID  int64     `json:"cardgroup_id"`
	Created      time.Time `json:"created"`
	Updated      time.Time `json:"updated"`
}

type NewCardGroup struct {
	Name    string    `json:"name" validate:"required,min=1"`
	CardIds []int64   `json:"card_ids,omitempty"`
	UserIds []int64   `json:"user_ids"`
	Created time.Time `json:"created"`
	Updated time.Time `json:"updated"`
}

type NewRole struct {
	Name    string    `json:"name" validate:"required,fl_name,min=1"`
	Created time.Time `json:"created"`
	Updated time.Time `json:"updated"`
}

type NewUser struct {
	Name    string    `json:"name" validate:"required,fl_name,min=1"`
	RoleIds []int64   `json:"role_ids"`
	Created time.Time `json:"created"`
	Updated time.Time `json:"updated"`
}

type PageInfo struct {
	EndCursor       *int64 `json:"endCursor,omitempty"`
	HasNextPage     bool   `json:"hasNextPage"`
	HasPreviousPage bool   `json:"hasPreviousPage"`
	StartCursor     *int64 `json:"startCursor,omitempty"`
}

type Query struct {
}

type Role struct {
	ID      int64           `json:"id"`
	Name    string          `json:"name" validate:"required,fl_name,min=1"`
	Created time.Time       `json:"created"`
	Updated time.Time       `json:"updated"`
	Users   *UserConnection `json:"users" validate:"-"`
}

type RoleConnection struct {
	Edges      []*RoleEdge `json:"edges,omitempty" validate:"-"`
	Nodes      []*Role     `json:"nodes,omitempty" validate:"-"`
	PageInfo   *PageInfo   `json:"pageInfo"`
	TotalCount int         `json:"totalCount"`
}

type RoleEdge struct {
	Cursor int64 `json:"cursor"`
	Node   *Role `json:"node" validate:"-"`
}

type User struct {
	ID         int64                `json:"id"`
	Name       string               `json:"name" validate:"required,fl_name,min=1"`
	Created    time.Time            `json:"created"`
	Updated    time.Time            `json:"updated"`
	CardGroups *CardGroupConnection `json:"cardGroups" validate:"-"`
	Roles      *RoleConnection      `json:"roles" validate:"-"`
}

type UserConnection struct {
	Edges      []*UserEdge `json:"edges,omitempty" validate:"-"`
	Nodes      []*User     `json:"nodes,omitempty" validate:"-"`
	PageInfo   *PageInfo   `json:"pageInfo"`
	TotalCount int         `json:"totalCount"`
}

type UserEdge struct {
	Cursor int64 `json:"cursor"`
	Node   *User `json:"node" validate:"-"`
}
