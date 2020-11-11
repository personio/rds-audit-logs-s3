# rds-audit-logs-s3

## Building and packaging the project

```
make package
```


## Releasing a new version

Create a new version tag with git and push the tag to Github:
```
git tag vx.x.x
git push origin vx.x.x
```

A new release in Github will automatically be created and the code will be published to the AWS Serverless Application Repository
