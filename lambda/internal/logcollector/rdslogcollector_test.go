package logcollector

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/rds/rdsiface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockRdsClient struct {
	rdsiface.RDSAPI
	mock.Mock
}

func (m *mockRdsClient) DownloadDBLogFilePortion(input *rds.DownloadDBLogFilePortionInput) (*rds.DownloadDBLogFilePortionOutput, error) {
	args := m.Called(input)
	return args.Get(0).(*rds.DownloadDBLogFilePortionOutput), args.Error(1)
}

func (m *mockRdsClient) DescribeDBLogFilesPages(input *rds.DescribeDBLogFilesInput, callback func(output *rds.DescribeDBLogFilesOutput, lastPage bool) bool) error {
	args := m.Called(input, callback)
	return args.Error(0)
}

type mockHttpClient struct {
	HTTPClient
	mock.Mock
}

func (m *mockHttpClient) Do(input *http.Request) (*http.Response, error) {
	args := m.Called(input)
	return args.Get(0).(*http.Response), args.Error(1)
}

const (
	TestRdsInstanceIdentifier = "my-rds-instance"
)

func TestFindLogFileNewerThanTimestamp(t *testing.T) {

	logFiles := []LogFile{
		{
			LastWritten: 1595262837000,
			LogFileName: "audit/server_audit.log",
			Size:        901862,
		},
		{
			LastWritten: 1595259824000,
			LogFileName: "audit/server_audit.log.1",
			Size:        1000159,
		},
		{
			LastWritten: 1595256406000,
			LogFileName: "audit/server_audit.log.2",
			Size:        1000011,
		},
		{
			LastWritten: 1595253008000,
			LogFileName: "audit/server_audit.log.3",
			Size:        1000022,
		},
	}

	log, err := findLogFileNewerThanTimestamp(logFiles, 1595253008000)
	assert.NoError(t, err)
	assert.Equal(t, int64(1595256406000), log.LastWritten)

	logZero, err := findLogFileNewerThanTimestamp(logFiles, 0)
	assert.NoError(t, err)
	assert.Equal(t, int64(1595253008000), logZero.LastWritten)

	logNonRotated, err := findLogFileNewerThanTimestamp(logFiles, 1595259824000)
	assert.NoError(t, err)
	assert.Nil(t, logNonRotated)
}

func TestGetLogFiles(t *testing.T) {
	rdsClient := new(mockRdsClient)
	httpClient := new(mockHttpClient)
	collector := NewRdsLogCollector(rdsClient, httpClient, "eu-central-1", TestRdsInstanceIdentifier, "mysql")

	ddlfInput := &rds.DescribeDBLogFilesInput{
		DBInstanceIdentifier: aws.String(TestRdsInstanceIdentifier),
	}
	ddlfOutput := &rds.DescribeDBLogFilesOutput{
		DescribeDBLogFiles: []*rds.DescribeDBLogFilesDetails{
			{
				LastWritten: aws.Int64(1595262837000),
				LogFileName: aws.String("audit/server_audit.log"),
				Size:        aws.Int64(901862),
			},
			{
				LastWritten: aws.Int64(1595259824000),
				LogFileName: aws.String("audit/server_audit.log.1"),
				Size:        aws.Int64(1000159),
			},
			{
				LastWritten: aws.Int64(1595256406000),
				LogFileName: aws.String("audit/server_audit.log.2"),
				Size:        aws.Int64(1000011),
			},
			{
				LastWritten: aws.Int64(1595261400000),
				LogFileName: aws.String("error/mysql-error-running.log"),
				Size:        aws.Int64(228),
			},
			{
				LastWritten: aws.Int64(1595236200000),
				LogFileName: aws.String("error/mysql-error-running.log.10"),
				Size:        aws.Int64(227),
			},
			{
				LastWritten: aws.Int64(1595262600000),
				LogFileName: aws.String("error/mysql-error.log"),
				Size:        aws.Int64(0),
			},
			{
				LastWritten: aws.Int64(1594656137000),
				LogFileName: aws.String("mysqlUpgrade"),
				Size:        aws.Int64(3337),
			},
		},
		Marker: nil,
	}
	rdsClient.On("DescribeDBLogFilesPages", ddlfInput, mock.AnythingOfType("func(*rds.DescribeDBLogFilesOutput, bool) bool")).Return(nil).Run(func(args mock.Arguments) {
		cb := args.Get(1).(func(*rds.DescribeDBLogFilesOutput, bool) bool)
		cb(ddlfOutput, true)
	})

	logFiles, err := collector.getLogFiles(maxRetries)
	assert.NoError(t, err)

	expectedLogfiles := []LogFile{
		{
			Size:            901862,
			LogFileName:     "audit/server_audit.log",
			LastWritten:     1595262837000,
			LastWrittenTime: time.Unix(1595262837000/1000, 0),
			Path:            "",
		},
		{
			Size:            1000159,
			LogFileName:     "audit/server_audit.log.1",
			LastWritten:     1595259824000,
			LastWrittenTime: time.Unix(1595259824000/1000, 0),
			Path:            "",
		},
		{
			Size:            1000011,
			LogFileName:     "audit/server_audit.log.2",
			LastWritten:     1595256406000,
			LastWrittenTime: time.Unix(1595256406000/1000, 0),
			Path:            "",
		},
	}

	assert.Equal(t, expectedLogfiles, logFiles)
	rdsClient.AssertExpectations(t)
}

func TestGetLogFilesWithRetry(t *testing.T) {
	rdsClient := new(mockRdsClient)
	httpClient := new(mockHttpClient)
	collector := NewRdsLogCollector(rdsClient, httpClient, "eu-central-1", TestRdsInstanceIdentifier, "mysql")

	ddlfInput := &rds.DescribeDBLogFilesInput{
		DBInstanceIdentifier: aws.String(TestRdsInstanceIdentifier),
	}
	ddlfOutput := &rds.DescribeDBLogFilesOutput{
		DescribeDBLogFiles: []*rds.DescribeDBLogFilesDetails{
			{
				LastWritten: aws.Int64(1595262837000),
				LogFileName: aws.String("audit/server_audit.log"),
				Size:        aws.Int64(901862),
			},
			{
				LastWritten: aws.Int64(1595259824000),
				LogFileName: aws.String("audit/server_audit.log.1"),
				Size:        aws.Int64(1000159),
			},
		},
		Marker: nil,
	}

	ddlfOutputEmpty := &rds.DescribeDBLogFilesOutput{
		DescribeDBLogFiles: []*rds.DescribeDBLogFilesDetails{},
		Marker:             nil,
	}

	rdsClient.On("DescribeDBLogFilesPages", ddlfInput, mock.AnythingOfType("func(*rds.DescribeDBLogFilesOutput, bool) bool")).Return(nil).Run(func(args mock.Arguments) {
		cb := args.Get(1).(func(*rds.DescribeDBLogFilesOutput, bool) bool)
		cb(ddlfOutputEmpty, true)
	}).Once()

	rdsClient.On("DescribeDBLogFilesPages", ddlfInput, mock.AnythingOfType("func(*rds.DescribeDBLogFilesOutput, bool) bool")).Return(nil).Run(func(args mock.Arguments) {
		cb := args.Get(1).(func(*rds.DescribeDBLogFilesOutput, bool) bool)
		cb(ddlfOutput, true)
	})

	logFiles, err := collector.getLogFiles(maxRetries)
	assert.NoError(t, err)

	expectedLogfiles := []LogFile{
		{
			Size:            901862,
			LogFileName:     "audit/server_audit.log",
			LastWritten:     1595262837000,
			LastWrittenTime: time.Unix(1595262837000/1000, 0),
			Path:            "",
		},
		{
			Size:            1000159,
			LogFileName:     "audit/server_audit.log.1",
			LastWritten:     1595259824000,
			LastWrittenTime: time.Unix(1595259824000/1000, 0),
			Path:            "",
		},
	}

	assert.Equal(t, expectedLogfiles, logFiles)
	rdsClient.AssertExpectations(t)
}

func TestGetLogsZeroTimestamp(t *testing.T) {
	rdsClient := new(mockRdsClient)
	httpClient := new(mockHttpClient)
	collector := NewRdsLogCollector(rdsClient, httpClient, "eu-central-1", TestRdsInstanceIdentifier, "mysql")

	ddlfInput := &rds.DescribeDBLogFilesInput{
		DBInstanceIdentifier: aws.String(TestRdsInstanceIdentifier),
	}
	ddlfOutput := &rds.DescribeDBLogFilesOutput{
		DescribeDBLogFiles: []*rds.DescribeDBLogFilesDetails{
			{
				LastWritten: aws.Int64(1595262837000),
				LogFileName: aws.String("audit/server_audit.log"),
				Size:        aws.Int64(901862),
			},
			{
				LastWritten: aws.Int64(1595259824000),
				LogFileName: aws.String("audit/server_audit.log.1"),
				Size:        aws.Int64(1000159),
			},
			{
				LastWritten: aws.Int64(1595256406000),
				LogFileName: aws.String("audit/server_audit.log.2"),
				Size:        aws.Int64(1000011),
			},
		},
		Marker: nil,
	}
	rdsClient.On("DescribeDBLogFilesPages", ddlfInput, mock.AnythingOfType("func(*rds.DescribeDBLogFilesOutput, bool) bool")).Return(nil).Run(func(args mock.Arguments) {
		cb := args.Get(1).(func(*rds.DescribeDBLogFilesOutput, bool) bool)
		cb(ddlfOutput, true)
	})

	logFileData1 := "20200720 16:37:59,ip-172-27-1-97,admin,10.120.186.117,305230,1337972,QUERY,rdslogstest,'SELECT \"Service fstehle-rdslog-test running: Mon Jul 20 16:37:59 UTC 2020 (947)\"',0\n20200720 16:37:59,ip-172-27-1-97,admin,10.120.186.117,305230,0,DISCONNECT,rdslogstest,,0\n20200720 16:38:00,ip-172-27-1-97,rdsadmin,localhost,26,1337974,QUERY,mysql,'SELECT 1',0\n20200720 16:38:00,ip-172-27-1-97,rdsadmin,localhost,26,47141561040897,QUERY,mysql,'SELECT count(*) from mysql.rds_history WHERE action = \\'disable set master\\' ORDER BY action_timestamp LIMIT 1',0\n20200720 16:38:00,ip-172-27-1-97,rdsadmin,localhost,26,1337976,QUERY,mysql,'SELECT 1',0\n20200720 16:38:00,ip-172-27-1-97,rdsadmin,localhost,26,47141561040897,QUERY,mysql,'SELECT count(*) from mysql.rds_replication_status WHERE master_host IS NOT NULL and master_port IS NOT NULL ORDER BY action_timestamp LIMIT 1',0\n20200720 16:38:01,ip-172-27-1-97,admin,10.120.186.117,305231,0,CONNECT,rdslogstest,,0\n20200720 16:38:01,ip-172-27-1-97,admin,10.120.186.117,305231,1337978,QUERY,rdslogstest,'select @@version_comment limit 1',0\n20200720 16:38:01,ip-172-27-1-97,admin,10.120.186.117,305231,1337979,QUERY,rdslogstest,'SELECT \"Service fstehle-rdslog-test running: Mon Jul 20 16:38:01 UTC 2020 (948)\"',0\n20200720 16:38:01,ip-172-27-1-97,admin,10.120.186.117,305231,0,DISCONNECT,rdslogstest,,0\n"

	doInput1 := mock.MatchedBy(func(i *http.Request) bool {
		return i.URL.Path == fmt.Sprintf("/v13/downloadCompleteLogFile/%s/%s", TestRdsInstanceIdentifier, "audit/server_audit.log.2")
	})
	httpClient.On("Do", doInput1).Return(&http.Response{
		Body:       ioutil.NopCloser(strings.NewReader(logFileData1)),
		StatusCode: 200,
	}, nil)

	logLines, _, logFileTimestamp, err := collector.GetLogs(int64(0))
	assert.NoError(t, err)
	logLinesBytes, _ := ioutil.ReadAll(logLines)
	assert.Equal(t, int64(1595256406000), logFileTimestamp)
	assert.Equal(t, logFileData1, string(logLinesBytes))

	rdsClient.AssertExpectations(t)
}

func TestGetLogsWithTimestamp(t *testing.T) {
	rdsClient := new(mockRdsClient)
	httpClient := new(mockHttpClient)
	collector := NewRdsLogCollector(rdsClient, httpClient, "eu-central-1", TestRdsInstanceIdentifier, "mysql")

	ddlfInput := &rds.DescribeDBLogFilesInput{
		DBInstanceIdentifier: aws.String(TestRdsInstanceIdentifier),
	}
	ddlfOutput := &rds.DescribeDBLogFilesOutput{
		DescribeDBLogFiles: []*rds.DescribeDBLogFilesDetails{
			{
				LastWritten: aws.Int64(1595262837000),
				LogFileName: aws.String("audit/server_audit.log"),
				Size:        aws.Int64(901862),
			},
			{
				LastWritten: aws.Int64(1595259824000),
				LogFileName: aws.String("audit/server_audit.log.1"),
				Size:        aws.Int64(1000159),
			},
			{
				LastWritten: aws.Int64(1595256406000),
				LogFileName: aws.String("audit/server_audit.log.2"),
				Size:        aws.Int64(1000011),
			},
		},
		Marker: nil,
	}
	rdsClient.On("DescribeDBLogFilesPages", ddlfInput, mock.AnythingOfType("func(*rds.DescribeDBLogFilesOutput, bool) bool")).Return(nil).Run(func(args mock.Arguments) {
		cb := args.Get(1).(func(*rds.DescribeDBLogFilesOutput, bool) bool)
		cb(ddlfOutput, true)
	})

	logFileData := "20200720 16:37:59,ip-172-27-1-97,admin,10.120.186.117,305230,1337972,QUERY,rdslogstest,'SELECT \"Service fstehle-rdslog-test running: Mon Jul 20 16:37:59 UTC 2020 (947)\"',0\n20200720 16:37:59,ip-172-27-1-97,admin,10.120.186.117,305230,0,DISCONNECT,rdslogstest,,0\n20200720 16:38:00,ip-172-27-1-97,rdsadmin,localhost,26,1337974,QUERY,mysql,'SELECT 1',0\n20200720 16:38:00,ip-172-27-1-97,rdsadmin,localhost,26,47141561040897,QUERY,mysql,'SELECT count(*) from mysql.rds_history WHERE action = \\'disable set master\\' ORDER BY action_timestamp LIMIT 1',0\n20200720 16:38:00,ip-172-27-1-97,rdsadmin,localhost,26,1337976,QUERY,mysql,'SELECT 1',0\n20200720 16:38:00,ip-172-27-1-97,rdsadmin,localhost,26,47141561040897,QUERY,mysql,'SELECT count(*) from mysql.rds_replication_status WHERE master_host IS NOT NULL and master_port IS NOT NULL ORDER BY action_timestamp LIMIT 1',0\n20200720 16:38:01,ip-172-27-1-97,admin,10.120.186.117,305231,0,CONNECT,rdslogstest,,0\n20200720 16:38:01,ip-172-27-1-97,admin,10.120.186.117,305231,1337978,QUERY,rdslogstest,'select @@version_comment limit 1',0\n20200720 16:38:01,ip-172-27-1-97,admin,10.120.186.117,305231,1337979,QUERY,rdslogstest,'SELECT \"Service fstehle-rdslog-test running: Mon Jul 20 16:38:01 UTC 2020 (948)\"',0\n20200720 16:38:01,ip-172-27-1-97,admin,10.120.186.117,305231,0,DISCONNECT,rdslogstest,,0\n"

	doInput1 := mock.MatchedBy(func(i *http.Request) bool {
		return i.URL.Path == fmt.Sprintf("/v13/downloadCompleteLogFile/%s/%s", TestRdsInstanceIdentifier, "audit/server_audit.log.1")
	})
	httpClient.On("Do", doInput1).Return(&http.Response{
		Body:       ioutil.NopCloser(strings.NewReader(logFileData)),
		StatusCode: 200,
	}, nil)

	logLines, _, currentMarker, err := collector.GetLogs(int64(1595256406000))
	assert.NoError(t, err)
	logLinesBytes, _ := ioutil.ReadAll(logLines)
	assert.Equal(t, int64(1595259824000), currentMarker)
	assert.Equal(t, logFileData, string(logLinesBytes))

	rdsClient.AssertExpectations(t)
}

func TestGetLogsWithTimestampRotatedInBetween(t *testing.T) {
	rdsClient := new(mockRdsClient)
	httpClient := new(mockHttpClient)
	collector := NewRdsLogCollector(rdsClient, httpClient, "eu-central-1", TestRdsInstanceIdentifier, "mysql")

	ddlfInput := &rds.DescribeDBLogFilesInput{
		DBInstanceIdentifier: aws.String(TestRdsInstanceIdentifier),
	}
	ddlfOutput := &rds.DescribeDBLogFilesOutput{
		DescribeDBLogFiles: []*rds.DescribeDBLogFilesDetails{
			{
				LastWritten: aws.Int64(1595262837000),
				LogFileName: aws.String("audit/server_audit.log"),
				Size:        aws.Int64(901862),
			},
			{
				LastWritten: aws.Int64(1595259824000),
				LogFileName: aws.String("audit/server_audit.log.1"),
				Size:        aws.Int64(1000159),
			},
			{
				LastWritten: aws.Int64(1595256406000),
				LogFileName: aws.String("audit/server_audit.log.2"),
				Size:        aws.Int64(1000011),
			},
		},
		Marker: nil,
	}
	rdsClient.On("DescribeDBLogFilesPages", ddlfInput, mock.AnythingOfType("func(*rds.DescribeDBLogFilesOutput, bool) bool")).Return(nil).Run(func(args mock.Arguments) {
		cb := args.Get(1).(func(*rds.DescribeDBLogFilesOutput, bool) bool)
		cb(ddlfOutput, true)
	}).Once()

	// Log files after rotation
	ddlfOutputRotated := &rds.DescribeDBLogFilesOutput{
		DescribeDBLogFiles: []*rds.DescribeDBLogFilesDetails{
			{
				LastWritten: aws.Int64(1595265641000),
				LogFileName: aws.String("audit/server_audit.log"),
				Size:        aws.Int64(901862),
			},
			{
				LastWritten: aws.Int64(1595262837000),
				LogFileName: aws.String("audit/server_audit.log.1"),
				Size:        aws.Int64(901862),
			},
			{
				LastWritten: aws.Int64(1595259824000),
				LogFileName: aws.String("audit/server_audit.log.2"),
				Size:        aws.Int64(1000159),
			},
			{
				LastWritten: aws.Int64(1595256406000),
				LogFileName: aws.String("audit/server_audit.log.3"),
				Size:        aws.Int64(1000011),
			},
		},
		Marker: nil,
	}
	rdsClient.On("DescribeDBLogFilesPages", ddlfInput, mock.AnythingOfType("func(*rds.DescribeDBLogFilesOutput, bool) bool")).Return(nil).Run(func(args mock.Arguments) {
		cb := args.Get(1).(func(*rds.DescribeDBLogFilesOutput, bool) bool)
		cb(ddlfOutputRotated, true)
	})

	logFileData := "20200720 16:37:59,ip-172-27-1-97,admin,10.120.186.117,305230,1337972,QUERY,rdslogstest,'SELECT \"Service fstehle-rdslog-test running: Mon Jul 20 16:37:59 UTC 2020 (947)\"',0\n20200720 16:37:59,ip-172-27-1-97,admin,10.120.186.117,305230,0,DISCONNECT,rdslogstest,,0\n20200720 16:38:00,ip-172-27-1-97,rdsadmin,localhost,26,1337974,QUERY,mysql,'SELECT 1',0\n20200720 16:38:00,ip-172-27-1-97,rdsadmin,localhost,26,47141561040897,QUERY,mysql,'SELECT count(*) from mysql.rds_history WHERE action = \\'disable set master\\' ORDER BY action_timestamp LIMIT 1',0\n20200720 16:38:00,ip-172-27-1-97,rdsadmin,localhost,26,1337976,QUERY,mysql,'SELECT 1',0\n20200720 16:38:00,ip-172-27-1-97,rdsadmin,localhost,26,47141561040897,QUERY,mysql,'SELECT count(*) from mysql.rds_replication_status WHERE master_host IS NOT NULL and master_port IS NOT NULL ORDER BY action_timestamp LIMIT 1',0\n20200720 16:38:01,ip-172-27-1-97,admin,10.120.186.117,305231,0,CONNECT,rdslogstest,,0\n20200720 16:38:01,ip-172-27-1-97,admin,10.120.186.117,305231,1337978,QUERY,rdslogstest,'select @@version_comment limit 1',0\n20200720 16:38:01,ip-172-27-1-97,admin,10.120.186.117,305231,1337979,QUERY,rdslogstest,'SELECT \"Service fstehle-rdslog-test running: Mon Jul 20 16:38:01 UTC 2020 (948)\"',0\n20200720 16:38:01,ip-172-27-1-97,admin,10.120.186.117,305231,0,DISCONNECT,rdslogstest,,0\n"

	doInput1 := mock.MatchedBy(func(i *http.Request) bool {
		return i.URL.Path == fmt.Sprintf("/v13/downloadCompleteLogFile/%s/%s", TestRdsInstanceIdentifier, "audit/server_audit.log.1")
	})
	httpClient.On("Do", doInput1).Return(&http.Response{
		Body:       ioutil.NopCloser(strings.NewReader(logFileData)),
		StatusCode: 200,
	}, nil)

	// Log files after rotation
	doInput2 := mock.MatchedBy(func(i *http.Request) bool {
		return i.URL.Path == fmt.Sprintf("/v13/downloadCompleteLogFile/%s/%s", TestRdsInstanceIdentifier, "audit/server_audit.log.2")
	})
	httpClient.On("Do", doInput2).Return(&http.Response{
		Body:       ioutil.NopCloser(strings.NewReader(logFileData)),
		StatusCode: 200,
	}, nil)

	logLines, _, currentMarker, err := collector.GetLogs(int64(1595256406000))
	assert.NoError(t, err)
	logLinesBytes, _ := ioutil.ReadAll(logLines)
	assert.Equal(t, int64(1595259824000), currentMarker)
	assert.Equal(t, logFileData, string(logLinesBytes))

	rdsClient.AssertExpectations(t)
}
