package logcollector

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/rds/rdsiface"
	log "github.com/sirupsen/logrus"
)

const maxRetries = 5

// LogFile wraps the returned structure from AWS
// "Size": 2196,
// "LogFileName": "slowquery/mysql-slowquery.log.7",
// "LastWritten": 1474959300000
type LogFile struct {
	Size            int64 // in bytes?
	LogFileName     string
	LastWritten     int64 // arrives as msec since epoch
	LastWrittenTime time.Time
	Path            string
}

func (l *LogFile) String() string {
	return fmt.Sprintf("%-35s (date: %s, size: %d)", l.LogFileName, l.LastWrittenTime, l.Size)
}

func (l *LogFile) IsRotatedFile() bool {
	matched, err := regexp.Match(`\.log\.\d+$`, []byte(l.LogFileName))
	if err != nil {
		log.Warnf("Error matching log file: %v", err)
		return false
	}
	return matched
}

// RdsLogCollectorOld contains handles to the provided LogCollectorOptions + aws.RDS struct
type RdsLogCollector struct {
	rds                rdsiface.RDSAPI
	region             string
	httpClient         HTTPClient
	instanceIdentifier string
	dbType             string
	logType            string
	logFile            string
}

func NewRdsLogCollector(api rdsiface.RDSAPI, httpClient HTTPClient, region string, rdsInstanceIdentifier string, dbType string) *RdsLogCollector {
	return &RdsLogCollector{
		rds:                api,
		region:             region,
		httpClient:         httpClient,
		dbType:             dbType,
		logFile:            "audit/server_audit.log",
		instanceIdentifier: rdsInstanceIdentifier,
	}
}

func (c *RdsLogCollector) GetLogs(logFileTimestamp int64) (io.Reader, bool, int64, error) {
	return c.getLogs(logFileTimestamp, maxRetries)
}

func (c *RdsLogCollector) ValidateAndPrepareRDSInstance() error {
	output, err := c.rds.DescribeDBInstances(&rds.DescribeDBInstancesInput{
		DBInstanceIdentifier: aws.String(c.instanceIdentifier),
		MaxRecords:           aws.Int64(20),
	})

	if err != nil {
		return fmt.Errorf("could not describe db instance: %v", err)
	}

	if len(output.DBInstances) == 0 {
		return fmt.Errorf("could not find db instance: %v", c.instanceIdentifier)
	}

	err = c.setRdsInstanceDBType(output.DBInstances[0])
	if err != nil {
		return fmt.Errorf("could not set db instance type: %v", err)
	}

	return nil
}

func (c *RdsLogCollector) getLogs(logFileTimestamp int64, retries int) (io.Reader, bool, int64, error) {
	currentLogFile, err := c.getCurrentLogFileNewerThanTimestamp(logFileTimestamp)

	if err != nil {
		return nil, false, 0, fmt.Errorf("could not get current log file: %v", err)
	}
	if currentLogFile == nil {
		// No newer logs are available
		return nil, false, 0, nil
	}

	log.WithField("logfile_timestamp", logFileTimestamp).WithField("logfile_name", currentLogFile.LogFileName).Info("Getting logs")

	resp, err := c.downloadLogFile(*currentLogFile)
	if err != nil {
		return nil, false, 0, fmt.Errorf("could not get log data: %v", err)
	}
	defer resp.Close()

	// Check if file was not rotated in the meantime
	newCurrentLogFile, err := c.getCurrentLogFileNewerThanTimestamp(logFileTimestamp)
	if err != nil {
		return nil, false, 0, fmt.Errorf("could not get current log file: %v", err)
	}
	if newCurrentLogFile.LogFileName != currentLogFile.LogFileName {
		// File was rotated in the meantime -> retry it
		if retries >= 1 {
			return c.getLogs(logFileTimestamp, retries-1)
		}
		return nil, false, 0, fmt.Errorf("file was rotated when getting the logs")
	}

	//_, err = io.Copy(buf, resp)
	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(resp)
	if err != nil {
		if retries >= 1 {
			log.WithField("logfile_timestamp", logFileTimestamp).WithField("retries", retries-1).Warn("Retrying because of error reading response body")
			time.Sleep(1 * time.Second)
			return c.getLogs(logFileTimestamp, retries-1)
		}
		return nil, false, 0, fmt.Errorf("could not read response from log data: %v", err)
	}

	return buf, true, currentLogFile.LastWritten, nil
}

// downloadLogFile will download a full RDS log at once from the AWS
// REST API Endpoint that is not available through the Go SDK.
// It will return an absolute string path to the file.
func (c *RdsLogCollector) downloadLogFile(currentLogFile LogFile) (io.ReadCloser, error) {
	client := c.httpClient
	host := fmt.Sprintf("https://rds.%s.amazonaws.com", c.region)

	req, err := http.NewRequest("GET", host, nil)
	if err != nil {
		return nil, err
	}

	req.URL.Path = fmt.Sprintf("/v13/downloadCompleteLogFile/%s/%s", c.instanceIdentifier, currentLogFile.LogFileName)
	req.Close = true

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("could not download log file %s, status code is %d", currentLogFile.LogFileName, resp.StatusCode)
	}

	fmt.Printf("Download request completed with status: %d > %s \n", resp.StatusCode, req.URL.Path)

	//defer resp.Body.Close()

	return resp.Body, nil
}

func (c *RdsLogCollector) setRdsInstanceDBType(instance *rds.DBInstance) error {
	engine := *instance.Engine

	var dbType string
	switch engine {
	case "mariadb":
		dbType = "mysql"
	case "postgres":
		dbType = "postgres"
	default:
		return fmt.Errorf("unsupported engine %s", engine)
	}

	c.dbType = dbType
	return nil
}

func (c *RdsLogCollector) getCurrentLogFileNewerThanTimestamp(logFileTimestamp int64) (*LogFile, error) {
	logFiles, err := c.getLogFiles(maxRetries)
	if err != nil {
		return nil, fmt.Errorf("cannot get log files: %v", err)
	}

	return findLogFileNewerThanTimestamp(logFiles, logFileTimestamp)
}

// getLogFiles returns a list of all log files based on the LogCollectorOptions.LogFile pattern
func (c *RdsLogCollector) getLogFiles(retries int) ([]LogFile, error) {
	var logFiles []LogFile

	err := c.rds.DescribeDBLogFilesPages(&rds.DescribeDBLogFilesInput{
		DBInstanceIdentifier: &c.instanceIdentifier,
	}, func(output *rds.DescribeDBLogFilesOutput, lastPage bool) bool {
		// assign go timestamp from msec epoch time, rebuild as a list
		for _, lf := range output.DescribeDBLogFiles {
			logFiles = append(logFiles, LogFile{
				LastWritten:     *lf.LastWritten,
				LastWrittenTime: time.Unix(*lf.LastWritten/1000, 0),
				LogFileName:     *lf.LogFileName,
				Size:            *lf.Size,
			})
		}

		return lastPage
	})
	if err != nil {
		return nil, fmt.Errorf("error getting db log files: %v", err)
	}

	var matchingLogFiles []LogFile
	for _, lf := range logFiles {
		if strings.HasPrefix(lf.LogFileName, c.logFile) {
			matchingLogFiles = append(matchingLogFiles, lf)
		}
	}
	// matchingLogFiles now contains a list of eligible log files,
	// eg slow.log, slow.log.1, slow.log.2, etc.

	if len(matchingLogFiles) == 0 {
		// sometimes the API returns empty results. Handle that with a retry to make sure it's really empty
		if retries >= 1 {
			return c.getLogFiles(retries - 1)
		}
		return nil, fmt.Errorf("No log file with the given prefix found. Number of log files: %v", len(logFiles))
	}

	return matchingLogFiles, nil
}

func findLogFileNewerThanTimestamp(logFiles []LogFile, finishedLogFileTimestamp int64) (*LogFile, error) {
	sort.SliceStable(logFiles, func(i, j int) bool { return logFiles[i].LastWritten < logFiles[j].LastWritten })

	for _, l := range logFiles {
		if l.LastWritten > finishedLogFileTimestamp && l.IsRotatedFile() {
			return &l, nil
		}
	}

	return nil, nil
}
