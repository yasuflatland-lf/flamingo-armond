package graph

import (
	"backend/graph/services"
	"backend/pkg/validator"

	"gorm.io/gorm"
)

// Resolver struct updated to include a DB field.
type Resolver struct {
	DB  *gorm.DB
	Srv services.Services
	VW  validator.ValidateWrapper
	*Loaders
}
