package logcollector

import (
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go/aws/session"
	v4 "github.com/aws/aws-sdk-go/aws/signer/v4"
)

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type AWSHttpClient struct {
	httpClient *http.Client
	signer     *v4.Signer
	region     string
}

func NewAWSHttpClient(sess *session.Session) *AWSHttpClient {

	return &AWSHttpClient{
		httpClient: &http.Client{},
		signer:     v4.NewSigner(sess.Config.Credentials),
		region:     *sess.Config.Region,
	}
}

func (a *AWSHttpClient) Do(req *http.Request) (*http.Response, error) {
	client := a.httpClient

	_, err := a.signer.Sign(req, nil, "rds", a.region, time.Now())
	if err != nil {
		return nil, err
	}

	return client.Do(req)
}
