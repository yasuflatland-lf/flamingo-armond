package repository

import "gorm.io/gorm"

// Tx is the transaction handle repositories accept. It is a type ALIAS (not a
// defined type) so every existing *gorm.DB value, closure, and test fixture
// stays assignable with zero behavior change; the usecase layer names
// repository.Tx and stops importing gorm directly.
type Tx = *gorm.DB
