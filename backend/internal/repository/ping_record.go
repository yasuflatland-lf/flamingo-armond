package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type gormPingRecord struct {
	ID        uuid.UUID `gorm:"column:id;primaryKey;type:uuid;default:gen_random_uuid()"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (gormPingRecord) TableName() string { return "ping_records" }

// PingRecordRepository provides read/write access to public.ping_records.
type PingRecordRepository interface {
	Count(ctx context.Context) (int64, error)
	Create(ctx context.Context) error
	DeleteAll(ctx context.Context) (int64, error)
}

type pingRecordRepo struct{ db *gorm.DB }

func NewPingRecordRepository(db *gorm.DB) PingRecordRepository {
	return &pingRecordRepo{db: db}
}

func (r *pingRecordRepo) Count(ctx context.Context) (int64, error) {
	var n int64
	if err := r.db.WithContext(ctx).Model(&gormPingRecord{}).Count(&n).Error; err != nil {
		return 0, err
	}
	return n, nil
}

// Create inserts a single ping record; the DB supplies id and timestamps.
func (r *pingRecordRepo) Create(ctx context.Context) error {
	return r.db.WithContext(ctx).Create(&gormPingRecord{}).Error
}

// DeleteAll removes all rows and returns the number of rows affected.
func (r *pingRecordRepo) DeleteAll(ctx context.Context) (int64, error) {
	result := r.db.WithContext(ctx).Where("1 = 1").Delete(&gormPingRecord{})
	return result.RowsAffected, result.Error
}
