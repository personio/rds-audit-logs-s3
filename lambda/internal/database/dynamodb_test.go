package database

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"rdsauditlogss3/internal/entity"
	"strconv"
	"testing"
)

type mockDynamoDBClient struct {
	dynamodbiface.DynamoDBAPI
	mock.Mock
}

func (m *mockDynamoDBClient) PutItem(input *dynamodb.PutItemInput) (*dynamodb.PutItemOutput, error) {
	args := m.Called(input)
	return args.Get(0).(*dynamodb.PutItemOutput), args.Error(1)
}

func (m *mockDynamoDBClient) GetItem(input *dynamodb.GetItemInput) (*dynamodb.GetItemOutput, error) {
	args := m.Called(input)
	return args.Get(0).(*dynamodb.GetItemOutput), args.Error(1)
}

const (
	TestTableName = "my-table"
)

func TestStoreCheckpoint(t *testing.T) {
	dynamoDBClient := new(mockDynamoDBClient)
	db := NewDynamoDb(dynamoDBClient, TestTableName)

	someID := "1"
	someMarker := int64(2)

	expectedDynamoDBInput := &dynamodb.PutItemInput{
		TableName: aws.String(TestTableName),
		Item: map[string]*dynamodb.AttributeValue{
			"id":                {S: aws.String(someID)},
			"logfile_timestamp": {N: aws.String(strconv.Itoa(int(someMarker)))},
		},
	}
	dynamoDBClient.On("PutItem", expectedDynamoDBInput).Return(&dynamodb.PutItemOutput{}, nil)

	err := db.StoreCheckpoint(&entity.CheckpointRecord{
		Id:               someID,
		LogFileTimestamp: someMarker,
	})
	assert.NoError(t, err)
	dynamoDBClient.AssertExpectations(t)
}

func TestGetCheckpoint(t *testing.T) {
	dynamoDBClient := new(mockDynamoDBClient)
	db := NewDynamoDb(dynamoDBClient, TestTableName)

	someID := "1"
	someMarker := int64(2)

	expectedDynamoDBInput := &dynamodb.GetItemInput{
		TableName: aws.String(TestTableName),
		Key: map[string]*dynamodb.AttributeValue{
			"id": {S: &someID},
		},
	}
	expectedDynamoDBOuput := &dynamodb.GetItemOutput{
		Item: map[string]*dynamodb.AttributeValue{
			"id":                {S: &someID},
			"logfile_timestamp": {N: aws.String(strconv.Itoa(int(someMarker)))},
		},
	}
	dynamoDBClient.On("GetItem", expectedDynamoDBInput).Return(expectedDynamoDBOuput, nil)

	record, err := db.GetCheckpoint(someID)
	assert.NoError(t, err)
	assert.Equal(t, &entity.CheckpointRecord{
		Id:               someID,
		LogFileTimestamp: someMarker,
	}, record)
	dynamoDBClient.AssertExpectations(t)
}
