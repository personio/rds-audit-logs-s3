package s3writer

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/aws/aws-sdk-go/service/s3/s3manager/s3manageriface"
	log "github.com/sirupsen/logrus"
	"rdsauditlogss3/internal/entity"
	"io"
)

type s3Writer struct {
	uploader   s3manageriface.UploaderAPI
	bucketName string
	s3Prefix   string
}

func NewS3Writer(uploader s3manageriface.UploaderAPI, bucketName string, s3Prefix string) Writer {
	return &s3Writer{
		uploader:   uploader,
		bucketName: bucketName,
		s3Prefix:   s3Prefix,
	}
}

func (s *s3Writer) WriteLogEntry(data entity.LogEntry) error {
	key := generateKey(s.s3Prefix, data.Timestamp, data.LogFileTimestamp)

	err := s.upload(key, data.LogLine)
	if err != nil {
		return fmt.Errorf("could not upload file to S3: %v", err)
	}
	return nil
}

func (s *s3Writer) upload(key string, data io.Reader) error {
	// Upload the file to S3.
	_, err := s.uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(s.bucketName),
		Key:    aws.String(key),
		Body:   data,
	})
	if err != nil {
		return fmt.Errorf("failed to upload file, %v", err)
	}
	log.WithField("key", key).Info("File uploaded to S3")

	return nil
}

func generateKey(s3Prefix string, ts entity.LogEntryTimestamp, logFileTimestamp int64) string {
	datePart := fmt.Sprintf("year=%04d/month=%02d/day=%02d/hour=%02d", ts.Year, ts.Month, ts.Day, ts.Hour)
	filename := fmt.Sprintf("%d.log", logFileTimestamp)
	return fmt.Sprintf("%s/%s/%s", s3Prefix, datePart, filename)
}
