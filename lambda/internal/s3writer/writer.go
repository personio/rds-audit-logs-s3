package s3writer

import "rdsauditlogss3/internal/entity"

// Writer is the interface for writing log entries to S3
type Writer interface {
	WriteLogEntry(data entity.LogEntry) error
}
