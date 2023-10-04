package s3writer

import (
	"bytes"
	"fmt"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/aws/aws-sdk-go/service/s3/s3manager/s3manageriface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"rdsauditlogss3/internal/entity"
	"testing"
)

type mockS3Uploader struct {
	s3manageriface.UploaderAPI
	mock.Mock
}

func (m *mockS3Uploader) Upload(input *s3manager.UploadInput, _ ...func(uploader *s3manager.Uploader)) (*s3manager.UploadOutput, error) {
	args := m.Called(input)
	return args.Get(0).(*s3manager.UploadOutput), args.Error(1)
}

const (
	TestBucketName            = "my-bucket"
	TestS3Prefix              = "my-rds-instance/audit-logs"
)

func TestWriteLogEntry(t *testing.T) {
	s3Uploader := new(mockS3Uploader)
	client := NewS3Writer(s3Uploader, TestBucketName, TestS3Prefix)

	expectedS3Input := mock.MatchedBy(func(i *s3manager.UploadInput) bool {
		return *i.Bucket == TestBucketName && *i.Key == fmt.Sprintf("%s/year=2020/month=07/day=13/hour=14/1595494263000.log", TestS3Prefix)
	})

	s3Uploader.On("Upload", expectedS3Input).Return(&s3manager.UploadOutput{}, nil)
	err := client.WriteLogEntry(entity.LogEntry{
		Timestamp:        entity.NewLogEntryTimestamp(2020, 7, 13, 14),
		LogLine:          bytes.NewBufferString("20200713 14:18:10,ip-172-27-2-141,monolith-web,10.160.167.194,10739612,551067709,QUERY,personio,'select * from `job_positions` where (`job_positions`.`company_id` = ? or `job_positions`.`company_id` is null) and `company_id` = ? and `id` = ? limit 1',0"),
		LogFileTimestamp: int64(1595494263000),
	})
	assert.NoError(t, err)

	s3Uploader.AssertExpectations(t)
}