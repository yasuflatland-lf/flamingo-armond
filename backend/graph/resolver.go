package graph

import (
	"backend/graph/services"
	"backend/pkg/usecases"
	"backend/pkg/validator"

	"gorm.io/gorm"
)

// Resolver struct updated to include a DB field.
type Resolver struct {
	DB  *gorm.DB
	Srv services.Services
	U   usecases.Usecases
	VW  validator.ValidateWrapper
	*Loaders
}
