package processor

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"rdsauditlogss3/internal/database"
	"rdsauditlogss3/internal/entity"
	"rdsauditlogss3/internal/logcollector"
	parser "rdsauditlogss3/internal/parser"
	"rdsauditlogss3/internal/s3writer"
	"io"
	"strings"
	"testing"
)

const (
	TestRdsInstanceIdentifier = "my-instance"
)

type mockDatabase struct {
	database.Database
	mock.Mock
}

func (m *mockDatabase) StoreCheckpoint(record *entity.CheckpointRecord) error {
	args := m.Called(record)
	return args.Error(0)
}

func (m *mockDatabase) GetCheckpoint(id string) (*entity.CheckpointRecord, error) {
	args := m.Called(id)
	return args.Get(0).(*entity.CheckpointRecord), args.Error(1)
}

type mockLogCollector struct {
	logcollector.LogCollector
	mock.Mock
}

func (m *mockLogCollector) GetLogs(timestamp int64) (io.Reader, bool, int64, error) {
	args := m.Called(timestamp)
	if args.Get(0) == nil {
		return nil, args.Get(1).(bool), args.Get(2).(int64), args.Error(3)
	}
	return args.Get(0).(io.Reader), args.Get(1).(bool), args.Get(2).(int64), args.Error(3)
}

func (m *mockLogCollector) ValidateAndPrepareRDSInstance() error {
	args := m.Called()
	return args.Error(0)
}

type mockWriter struct {
	s3writer.Writer
	mock.Mock
}

func (m *mockWriter) WriteLogEntry(data entity.LogEntry) error {
	args := m.Called(data)
	return args.Error(0)
}

func TestProcessOneLogCallback(t *testing.T) {
	p := parser.NewAuditLogParser()
	db := new(mockDatabase)
	lc := new(mockLogCollector)
	w := new(mockWriter)

	id := fmt.Sprintf("%s:%s", TestRdsInstanceIdentifier, "audit")
	initialMarker := int64(0)
	nextMarker := int64(2)
	logLineDate := entity.NewLogEntryTimestamp(2020, 7, 14, 7)
	logLine := "20200714 07:05:25,ip-172-27-1-97,rdsadmin,localhost,26,47141561040897,QUERY,mysql,'SELECT NAME, VALUE FROM mysql.rds_configuration',0"

	db.On("GetCheckpoint", id).Return(&entity.CheckpointRecord{
		LogFileTimestamp: initialMarker,
		Id:               id,
	}, nil)
	db.On("StoreCheckpoint", &entity.CheckpointRecord{
		LogFileTimestamp: nextMarker,
		Id:               id,
	}).Return(nil)

	lc.On("ValidateAndPrepareRDSInstance").Return(nil)
	lc.On("GetLogs", initialMarker).Return(strings.NewReader(logLine), true, nextMarker, nil).Once()
	lc.On("GetLogs", nextMarker).Return(nil, false, int64(0), nil).Once()

	expectedWriteLogEntryInput := mock.MatchedBy(func(data entity.LogEntry) bool {
		return data.Timestamp == logLineDate && data.LogLine.String() == logLine+"\n" && data.LogFileTimestamp == nextMarker
	})
	w.On("WriteLogEntry", expectedWriteLogEntryInput).Return(nil)

	processor := NewProcessor(db, lc, w, p, TestRdsInstanceIdentifier)
	err := processor.Process()
	assert.NoError(t, err)

	db.AssertExpectations(t)
	lc.AssertExpectations(t)
	w.AssertExpectations(t)
}

func TestProcessMultiLogCallback(t *testing.T) {
	p := parser.NewAuditLogParser()
	db := new(mockDatabase)
	lc := new(mockLogCollector)
	w := new(mockWriter)

	id := fmt.Sprintf("%s:%s", TestRdsInstanceIdentifier, "audit")
	logFileTimestamp1 := int64(1)
	logFileTimestamp2 := int64(2)
	logFileTimestamp3 := int64(3)

	logLine1Date := entity.NewLogEntryTimestamp(2020, 7, 14, 7)
	logLine1 := "20200714 07:05:25,ip-172-27-1-97,rdsadmin,localhost,26,47141561040897,QUERY,mysql,'SELECT NAME, VALUE FROM mysql.rds_configuration',0"
	logLine2Date := entity.NewLogEntryTimestamp(2020, 7, 14, 8)
	logLine2 := "20200714 08:05:30,ip-172-27-1-97,rdsadmin,localhost,26,47141561040897,QUERY,mysql,'SELECT NAME, VALUE FROM mysql.rds_configuration',0"
	logLine3Date := entity.NewLogEntryTimestamp(2020, 7, 14, 9)
	logLine3 := "20200714 09:06:30,ip-172-27-1-97,rdsadmin,localhost,26,47141561040897,QUERY,mysql,'SELECT NAME, VALUE FROM mysql.rds_configuration',0"

	db.On("GetCheckpoint", id).Return(&entity.CheckpointRecord{
		LogFileTimestamp: logFileTimestamp1,
		Id:               id,
	}, nil)
	db.On("StoreCheckpoint", &entity.CheckpointRecord{
		LogFileTimestamp: logFileTimestamp2,
		Id:               id,
	}).Return(nil)
	db.On("StoreCheckpoint", &entity.CheckpointRecord{
		LogFileTimestamp: logFileTimestamp3,
		Id:               id,
	}).Return(nil)

	lc.On("ValidateAndPrepareRDSInstance").Return(nil)
	lc.On("GetLogs", logFileTimestamp1).Return(strings.NewReader(fmt.Sprintf("%s\n%s",logLine1,logLine2)), true, logFileTimestamp2, nil).Once()
	lc.On("GetLogs", logFileTimestamp2).Return(strings.NewReader(logLine3), true, logFileTimestamp3, nil).Once()
	lc.On("GetLogs", logFileTimestamp3).Return(nil, false, int64(0), nil).Once()


	expectedWriteLogEntryInput := mock.MatchedBy(func(data entity.LogEntry) bool {
		return data.Timestamp == logLine1Date && data.LogLine.String() == fmt.Sprintf("%s\n", logLine1) && data.LogFileTimestamp == logFileTimestamp2
	})
	w.On("WriteLogEntry", expectedWriteLogEntryInput).Return(nil)
	expectedWriteLogEntryInput2 := mock.MatchedBy(func(data entity.LogEntry) bool {
		return data.Timestamp == logLine2Date && data.LogLine.String() == fmt.Sprintf("%s\n", logLine2) && data.LogFileTimestamp == logFileTimestamp2
	})
	w.On("WriteLogEntry", expectedWriteLogEntryInput2).Return(nil)
	expectedWriteLogEntryInput3 := mock.MatchedBy(func(data entity.LogEntry) bool {
		return data.Timestamp == logLine3Date && data.LogLine.String() == fmt.Sprintf("%s\n", logLine3) && data.LogFileTimestamp == logFileTimestamp3
	})
	w.On("WriteLogEntry", expectedWriteLogEntryInput3).Return(nil)

	processor := NewProcessor(db, lc, w, p, TestRdsInstanceIdentifier)
	err := processor.Process()
	assert.NoError(t, err)

	db.AssertExpectations(t)
	lc.AssertExpectations(t)
	w.AssertExpectations(t)
}
