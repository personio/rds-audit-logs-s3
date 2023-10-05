package parser
import (
	"rdsauditlogss3/internal/entity"
	"io"
)
type Parser interface {
	ParseEntries(data io.Reader, logFileTimestamp int64) ([]*entity.LogEntry, error)
}
