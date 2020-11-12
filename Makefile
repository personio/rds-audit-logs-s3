SAM                    := sam
AWS_REGION             := eu-central-1
S3_BUCKET              := personio-oss-sar-rds-audit-logs-s3-$(AWS_REGION)
PACKAGED_TEMPLATE_FILE := packaged.yaml

# Run unit tests of Lamba function code
.PHONY: test
test:
	cd lambda && go test ./... -v -race -count=1 -cover $(PACKAGES) -coverprofile=coverage.out
	cd lambda && go tool cover -func=coverage.out

# Build Lambda function code
.PHONY: build
build:
	$(SAM) build

# Package AWS SAM application
.PHONY: package
package: build
	$(SAM) package --s3-bucket $(S3_BUCKET) --region $(AWS_REGION) --output-template-file $(PACKAGED_TEMPLATE_FILE)

# Publish packaged AWS SAM template to the AWS Serverless Application Repository
.PHONY: publish
publish: guard-VERSION
	$(SAM) publish --semantic-version $(VERSION) --template-file $(PACKAGED_TEMPLATE_FILE)

# Guard to make sure a variable is set
.PHONY: guard-%
guard-%:
	$(if $(value ${*}),,$(error "Variable ${*} not set!"))
