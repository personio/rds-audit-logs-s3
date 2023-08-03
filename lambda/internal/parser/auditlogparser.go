package parser

import (
	"bufio"
	"bytes"
	"fmt"
	"rdsauditlogss3/internal/entity"
	"io"
	"strings"
	"time"
)

type AuditLogParser struct {
}

func NewAuditLogParser() *AuditLogParser {
	return &AuditLogParser{}
}

func (p *AuditLogParser) ParseEntries(data io.Reader, logFileTimestamp int64) ([]*entity.LogEntry, error) {
	var entries []*entity.LogEntry
	var currentEntry *entity.LogEntry

	scanner := bufio.NewScanner(data)
	for scanner.Scan() {
		txt := scanner.Text()
		if txt == "" {
			continue
		}

		record := strings.Split(txt,",")

		if len(record) < 2 {
			return nil, fmt.Errorf("could not parse data")
		}

		timestamp, _ := strconv.ParseInt(record[0], 10, 64)
		epochSeconds := timestamp / 1000000
		t := time.Unix(epochSeconds, 0)
		formatTime := t.Format("20060102 15:04:05")

		ts, err := time.Parse("20060102 15:04:05", formatTime)

		if err != nil {
			return nil, fmt.Errorf("could not parse time: %v", err)
		}

		newTS := entity.LogEntryTimestamp{
			Year:  ts.Year(),
			Month: int(ts.Month()),
			Day:   ts.Day(),
			Hour:  ts.Hour(),
		}

		if currentEntry != nil && currentEntry.Timestamp != newTS {
			entries = append(entries, currentEntry)
			currentEntry = nil
		}

		if currentEntry == nil {
			currentEntry = &entity.LogEntry{
				Timestamp:        newTS,
				LogLine:          new(bytes.Buffer),
				LogFileTimestamp: logFileTimestamp,
			}
		}

		currentEntry.LogLine.WriteString(txt)
		currentEntry.LogLine.WriteString("\n")
	}

	entries = append(entries, currentEntry)

	return entries, nil
}
