package processor

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"rdsauditlogss3/internal/database"
	"rdsauditlogss3/internal/entity"
	"rdsauditlogss3/internal/logcollector"
	"rdsauditlogss3/internal/parser"
	"rdsauditlogss3/internal/s3writer"
)

type Processor struct {
	database              database.Database
	logcollector          logcollector.LogCollector
	S3Writer              s3writer.Writer
	Parser                parser.Parser
	RdsInstanceIdentifier string
}

func NewProcessor(db database.Database, lc logcollector.LogCollector, w s3writer.Writer, p parser.Parser, rdsInstanceIdentifier string) *Processor {
	return &Processor{
		database:              db,
		logcollector:          lc,
		S3Writer:              w,
		Parser:                p,
		RdsInstanceIdentifier: rdsInstanceIdentifier,
	}
}

func (p *Processor) Process() error {
	// Validate RDS instance
	err := p.logcollector.ValidateAndPrepareRDSInstance()
	if err != nil {
		return fmt.Errorf("error validating RDS instance: %v", err)
	}

	// Get current checkpoint from database
	id := fmt.Sprintf("%s:%s", p.RdsInstanceIdentifier, "audit")
	checkpointRecord, err := p.database.GetCheckpoint(id)
	if err != nil {
		return fmt.Errorf("could not get marker: %v", err)
	}

	currentLogFileTimestamp := int64(0)
	if checkpointRecord != nil {
		currentLogFileTimestamp = checkpointRecord.LogFileTimestamp
	}

	processedLogFiles := 0

	for {
		logLines, ok, logFileTimestamp, err := p.logcollector.GetLogs(currentLogFileTimestamp)
		if err != nil {
			return fmt.Errorf("could not start logcollector: %v", err)
		}
		if !ok {
			// No more logs available
			break
		}
		
		// d1 := []byte(logLines[0])
		// err = ioutil.WriteFile(fmt.Sprintf("/tmp/%d", logFileTimestamp), d1, 0644)

		currentLogFileTimestamp = logFileTimestamp

		logEntries, err := p.Parser.ParseEntries(logLines, currentLogFileTimestamp)
		if err != nil {
			logrus.WithFields(logrus.Fields{"err": err}).Warn("Could not parse entries")
			return fmt.Errorf("could not parse entries: %v", err)
		}

		for _, entry := range logEntries {
			processedLogFiles += 1

			err := p.S3Writer.WriteLogEntry(*entry)
			if err != nil {
				logrus.WithError(err).Warn("Could not write log entry")
				return fmt.Errorf("could not write log entry: %v", err)
			}
		}

		logrus.WithField("logfile_timestamp", currentLogFileTimestamp).Info("StoreCheckpoint")
		err = p.database.StoreCheckpoint(&entity.CheckpointRecord{
			LogFileTimestamp: currentLogFileTimestamp,
			Id:               id,
		})
		if err != nil {
			return fmt.Errorf("could not save marker: %v", err)
		}
	}

	logrus.WithFields(logrus.Fields{"processed_log_files": processedLogFiles}).Info("Processing logs is finished")

	return nil
}
