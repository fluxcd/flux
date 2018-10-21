package ecr

import (
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/ecr/ecriface"
)

type mockECRClient struct {
	ecriface.ECRAPI
	resp ecr.GetAuthorizationTokenOutput
}

// Mock the response
func (m *mockECRClient) GetAuthorizationToken(input *ecr.GetAuthorizationTokenInput) (*ecr.GetAuthorizationTokenOutput, error) {
	return &m.resp, nil
}

func TestGetAmazonECRToken(t *testing.T) {
	tc := struct {
		resp               ecr.GetAuthorizationTokenOutput
		expectedHost       string
		expectedCredential string
	}{
		// Mock the response
		resp: ecr.GetAuthorizationTokenOutput{
			AuthorizationData: []*ecr.AuthorizationData{
				{
					ProxyEndpoint:      aws.String("https://12345.dkr.ecr-use-east-1.amazonaws.com"),
					AuthorizationToken: aws.String("QVdTOmVjcnBhc3N3b3JkCg=="),
				},
			},
		},
		expectedHost:       "12345.dkr.ecr-use-east-1.amazonaws.com",
		expectedCredential: "QVdTOmVjcnBhc3N3b3JkCg==",
	}
	mockSvc := &mockECRClient{resp: tc.resp}
	cred, err := GetAmazonECRToken(mockSvc, []string{"12345", "6024011343452"})
	if err != nil {
		t.Fatal(err)
	}
	if _, found := cred.Auths[tc.expectedHost]; !found {
		t.Fatalf("expecting to see %s based from the mocked response", tc.expectedHost)
	}
	if cred.Auths[tc.expectedHost].Auth != tc.expectedCredential {
		t.Fatalf("expecting to see %s credential based from the mocked response", tc.expectedCredential)
	}
	fmt.Println("--- INFO: got ", cred)
}
