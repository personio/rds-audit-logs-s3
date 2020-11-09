package database

import "rdsauditlogss3/internal/entity"

// Database is the high-level interface for interacting with dynamodb
type Database interface {
	StoreCheckpoint(checkpoint *entity.CheckpointRecord) error
	GetCheckpoint(id string) (*entity.CheckpointRecord, error)
}
