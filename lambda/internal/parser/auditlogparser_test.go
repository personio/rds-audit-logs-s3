package parser

import (
	"github.com/stretchr/testify/assert"
	"rdsauditlogss3/internal/entity"
	"strings"
	"testing"
)

func TestWriteLogEntrySingleLine(t *testing.T) {
	parser := NewAuditLogParser()

	logFileTimestamp := int64(1595332052)
	logLine := "20200714 07:05:25,ip-172-27-1-97,rdsadmin,localhost,26,47141561040897,QUERY,mysql,'SELECT NAME, VALUE FROM mysql.rds_configuration',0"
	entries, err := parser.ParseEntries(strings.NewReader(logLine), logFileTimestamp)
	assert.NoError(t, err)

	assert.Equal(t, entity.NewLogEntryTimestamp(2020, 7, 14, 7), entries[0].Timestamp)
	assert.Equal(t, logLine + "\n", entries[0].LogLine.String())
	assert.Equal(t, logFileTimestamp, entries[0].LogFileTimestamp)
}

func TestWriteLogEntryMultiLine(t *testing.T) {
	parser := NewAuditLogParser()

	logFileTimestamp := int64(1595332052)
	logLine := `20200714 10:30:02,ip-172-27-1-97,admin,10.120.182.212,33303,0,CONNECT,rdslogstest,,0
20200714 10:30:02,ip-172-27-1-97,admin,10.120.182.212,33303,161152,QUERY,rdslogstest,'select @@version_comment limit 1',0
20200714 10:30:02,ip-172-27-1-97,admin,10.120.182.212,33303,161153,QUERY,rdslogstest,'SELECT "Service fstehle-rdslog-test running: Tue Jul 14 10:30:02 UTC 2020 (32932)"',0
20200714 10:30:02,ip-172-27-1-97,admin,10.120.182.212,33303,0,DISCONNECT,rdslogstest,,0
20200714 10:30:03,ip-172-27-1-97,rdsadmin,localhost,26,161155,QUERY,mysql,'SELECT 1',0
20200714 10:30:03,ip-172-27-1-97,rdsadmin,localhost,26,161156,QUERY,mysql,'SELECT 1',0
20200714 10:30:03,ip-172-27-1-97,rdsadmin,localhost,26,161157,QUERY,mysql,'SELECT 1',0
20200714 10:30:03,ip-172-27-1-97,rdsadmin,localhost,26,161159,QUERY,mysql,'SELECT count(*) from information_schema.TABLES WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?',0
20200714 10:30:03,ip-172-27-1-97,rdsadmin,localhost,26,161161,QUERY,mysql,'SELECT 1',0
20200714 10:30:03,ip-172-27-1-97,rdsadmin,localhost,26,47141561040897,QUERY,mysql,'SELECT value FROM mysql.rds_heartbeat2',0
20200714 10:30:03,ip-172-27-1-97,rdsadmin,localhost,26,161163,QUERY,mysql,'SELECT 1',0
20200714 10:30:03,ip-172-27-1-97,rdsadmin,localhost,26,161165,QUERY,mysql,'SELECT count(*) from information_schema.TABLES WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?',0
20200714 10:30:03,ip-172-27-1-97,rdsadmin,localhost,26,161167,QUERY,mysql,'SELECT 1',0
20200714 10:30:03,ip-172-27-1-97,rdsadmin,localhost,26,161169,QUERY,mysql,'INSERT INTO mysql.rds_heartbeat2(id, value) values (1,1594722603906) ON DUPLICATE KEY UPDATE value = 1594722603906',0
20200714 10:30:03,ip-172-27-1-97,rdsadmin,localhost,26,161171,QUERY,mysql,'SELECT 1',0
20200714 10:30:03,ip-172-27-1-97,rdsadmin,localhost,26,161172,QUERY,mysql,'COMMIT',0
20200714 11:30:04,ip-172-27-1-97,admin,10.120.182.212,33304,0,CONNECT,rdslogstest,,0
20200714 11:30:04,ip-172-27-1-97,admin,10.120.182.212,33304,161173,QUERY,rdslogstest,'select @@version_comment limit 1',0
20200714 11:30:04,ip-172-27-1-97,admin,10.120.182.212,33304,161174,QUERY,rdslogstest,'SELECT "Service fstehle-rdslog-test running: Tue Jul 14 10:30:04 UTC 2020 (32933)"',0
20200714 11:30:04,ip-172-27-1-97,admin,10.120.182.212,33304,0,DISCONNECT,rdslogstest,,0
20200714 11:30:06,ip-172-27-1-97,admin,10.120.182.212,33305,0,CONNECT,rdslogstest,,0
20200714 11:30:06,ip-172-27-1-97,admin,10.120.182.212,33305,161176,QUERY,rdslogstest,'select @@version_comment limit 1',0
20200714 11:30:06,ip-172-27-1-97,admin,10.120.182.212,33305,161177,QUERY,rdslogstest,'SELECT "Service fstehle-rdslog-test running: Tue Jul 14 10:30:06 UTC 2020 (32934)"',0
20200714 11:30:06,ip-172-27-1-97,admin,10.120.182.212,33305,0,DISCONNECT,rdslogstest,,0
20200714 12:30:03,ip-172-27-1-97,rdsadmin,localhost,26,161171,QUERY,mysql,'SELECT 1',0
`

	entries, err := parser.ParseEntries(strings.NewReader(logLine), logFileTimestamp)
	assert.NoError(t, err)
	assert.Equal(t, entity.NewLogEntryTimestamp(2020, 7, 14, 10), entries[0].Timestamp)
	assert.Equal(t, logFileTimestamp, entries[0].LogFileTimestamp)
	assert.Len(t, entries, 3)

	assert.Equal(t, "20200714 12:30:03,ip-172-27-1-97,rdsadmin,localhost,26,161171,QUERY,mysql,'SELECT 1',0" + "\n", entries[2].LogLine.String())
}
