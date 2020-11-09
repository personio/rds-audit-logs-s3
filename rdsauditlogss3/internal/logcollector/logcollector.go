package logcollector

import "io"

type LogCollector interface {
	GetLogs(logFileTimestamp int64) (io.Reader, bool, int64, error)
	ValidateAndPrepareRDSInstance() error
}

type GetLogsCallback func(logLine string, logFileTimestamp int64)
