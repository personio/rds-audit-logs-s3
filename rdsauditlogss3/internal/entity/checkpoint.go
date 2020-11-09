package entity

// CheckpointRecord is the data used for storing a checkpoint in dynamodb
type CheckpointRecord struct {
	LogFileTimestamp int64
	Id               string
}
