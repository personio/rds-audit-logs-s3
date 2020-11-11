package entity

import "bytes"

type LogEntryTimestamp struct {
	Year  int
	Month int
	Day   int
	Hour  int
}

func NewLogEntryTimestamp(year, month , day, hour int) LogEntryTimestamp {
	return LogEntryTimestamp{
		Year:  year,
		Month: month,
		Day:   day,
		Hour:  hour,
	}
}

type LogEntry struct {
	Timestamp        LogEntryTimestamp
	LogLine          *bytes.Buffer
	LogFileTimestamp int64
}
