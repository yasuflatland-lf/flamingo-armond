package graph

import "gorm.io/gorm"

// Resolver struct updated to include a DB field.
type Resolver struct {
	DB *gorm.DB
}
