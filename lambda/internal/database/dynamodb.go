package database

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
	"rdsauditlogss3/internal/entity"
)

// Internal checkpoint record for DynamoDB
type dynamoDBCheckpointRecord struct {
	LogFileTimestamp int64  `dynamodbav:"logfile_timestamp,omitempty"`
	Id               string `dynamodbav:"id,omitempty"`
}

// DatabaseDynamo persists checkpoints
type DatabaseDynamo struct {
	client    dynamodbiface.DynamoDBAPI
	tableName string
}

// NewDynamoDb creates a new *DatabaseDynamo
func NewDynamoDb(dynamoDBClient dynamodbiface.DynamoDBAPI, tableName string) *DatabaseDynamo {
	return &DatabaseDynamo{
		client:    dynamoDBClient,
		tableName: tableName,
	}
}

// StoreCheckpoint puts a checkpoint into the database
func (db *DatabaseDynamo) StoreCheckpoint(record *entity.CheckpointRecord) error {
	attributeValues, err := dynamodbattribute.MarshalMap(&dynamoDBCheckpointRecord{
		LogFileTimestamp: record.LogFileTimestamp,
		Id:               record.Id,
	})
	if err != nil {
		return fmt.Errorf("failed DynamoDB marshal Record: %v", err)
	}

	putItemInput := &dynamodb.PutItemInput{
		TableName: aws.String(db.tableName),
		Item:      attributeValues,
	}

	_, err = db.client.PutItem(putItemInput)
	if err != nil {
		return fmt.Errorf("failed to save checkpoint to dynamodb: %v", err)
	}

	return nil
}

// GetCheckpoint retrieves a checkpoint from the database
func (db *DatabaseDynamo) GetCheckpoint(id string) (*entity.CheckpointRecord, error) {
	out, err := db.client.GetItem(&dynamodb.GetItemInput{
		Key: map[string]*dynamodb.AttributeValue{
			"id": {S: aws.String(id)},
		},
		TableName: aws.String(db.tableName),
	})
	if err != nil {
		return nil, fmt.Errorf("error getting checkpoint from DynamoDB: %v", err)
	}

	if out.Item == nil {
		return nil, nil
	}

	var record dynamoDBCheckpointRecord
	err = dynamodbattribute.UnmarshalMap(out.Item, &record)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling record from DynamoDB: %v", err)
	}

	return &entity.CheckpointRecord{
		LogFileTimestamp: record.LogFileTimestamp,
		Id:               record.Id,
	}, nil
}
