package graph

import (
	"backend/graph/services"
	"gorm.io/gorm"
)

// Resolver struct updated to include a DB field.
type Resolver struct {
	DB  *gorm.DB
	Srv services.Services
}
