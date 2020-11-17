# rds-audit-logs-s3

Serverless application to ingest RDS audit logs from RDS to S3.

This application uses the following AWS resources:
* DynamoDB for storing timestamps
* Lambda for running the code
* S3 for storing the log data (not part of the application, must be provided as input)

An instance of the application has to be set up for each database you want to get audit logs for.
Each Lambda function is called periodically by Cloudwatch events.

The ingestion process is as follows:
1. Get latest timestamp (timestamp of latest processed log file) from DynamoDB
2. Use timestamp to get next audit log file (which must already be rotated)
3. Abort if no log file has been found
4. Get all log data for log file
5. Check if log file has been rotated in the meantime and retry if that is the case
6. Parse all log data for log file
7. Write log data to S3 (using the timestamp and the date as part of the key -> "Athena layout")
8. Save timestamp in DynamoDB
9. Continue at 2.

## Example setup using Terraform

```hcl-terraform
locals {
  sar_application         = "arn:aws:serverlessrepo:eu-central-1:640663510286:applications/rds-audit-logs-s3"
  sar_application_version = "0.0.2"
  rds_instance_identifier = "mydb"
}

resource "aws_cloudformation_stack" "rds-audit-logs" {
  name = "rds-audit-logs-${local.rds_instance_identifier}"

  template_body = file("${path.module}/cf_template.yaml")

  parameters = {
    Name                 = "rds-audit-logs-${local.rds_instance_identifier}"
    BucketName            = aws_s3_bucket.rds_audit_logs.id
    RdsInstanceIdentifier = local.rds_instance_identifier
    SarApplication        = local.sar_application
    SarApplicationVersion = local.sar_application_version
  }

  capabilities = ["CAPABILITY_AUTO_EXPAND", "CAPABILITY_IAM"]
}

resource "aws_s3_bucket" "rds_audit_logs" {
  bucket = "rds-audit-logs"
  acl    = "private"
}
```

```yaml
# cf_template.yaml
AWSTemplateFormatVersion: '2010-09-09'
Transform: AWS::Serverless-2016-10-31

Parameters:
  Name:
    Type: String
  BucketName:
    Type: String
  RdsInstanceIdentifier:
    Type: String
  SarApplication:
    Type: String
  SarApplicationVersion:
    Type: String

Resources:
  RdsAuditLogsS3Application:
    Type: AWS::Serverless::Application
    Properties:
      Location:
        ApplicationId: !Ref SarApplication
        SemanticVersion: !Ref SarApplicationVersion
      Parameters:
        Name: !Ref Name
        BucketName: !Ref BucketName
        RdsInstanceIdentifier: !Ref RdsInstanceIdentifier
      TimeoutInMinutes: 5
```

## Example setup using Cloudformation

```yaml
AWSTemplateFormatVersion: '2010-09-09'
Transform: AWS::Serverless-2016-10-31

Parameters:
  RdsInstanceIdentifier:
    Type: String
    Default: "mydb"
  SarApplication:
    Type: String
    Default: "arn:aws:serverlessrepo:eu-central-1:640663510286:applications/rds-audit-logs-s3"
  SarApplicationVersion:
    Type: String
    Default: "0.0.2"

Resources:
  RdsAuditLogsS3Application:
    Type: AWS::Serverless::Application
    Properties:
      Location:
        ApplicationId: !Ref SarApplication
        SemanticVersion: !Ref SarApplicationVersion
      Parameters:
        Name: !Sub "rds-audit-logs-${RdsInstanceIdentifier}"
        BucketName: !Ref S3Bucket
        RdsInstanceIdentifier: !Ref RdsInstanceIdentifier
      TimeoutInMinutes: 5
  S3Bucket:
    Type: AWS::S3::Bucket
    Properties:
      BucketName: "rds-audit-logs"
      AccessControl: "private"
```

## Development

### Requirements
* python3
* go 1.14

### Running the unit tests

```
make test
```

### Building and packaging the project

```
make package
```

### Releasing a new version

Create a new version tag with git and push the tag to Github:
```
git tag vx.x.x
git push origin vx.x.x
```

A new release in Github will automatically be created and the code will be published to the AWS Serverless Application Repository
