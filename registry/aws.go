package registry

// References:
//  - https://github.com/bzon/ecr-k8s-secret-creator
//  - https://github.com/kubernetes/kubernetes/blob/master/pkg/credentialprovider/aws/aws_credentials.go
//  - https://github.com/weaveworks/flux/pull/1455

import (
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/go-kit/kit/log"
)

const (
	// For recognising ECR hosts
	ecrHostSuffix = ".amazonaws.com"
	// How long AWS tokens remain valid
	tokenValid = 12 * time.Hour
)

type AWSRegistryConfig struct {
	Region      string
	RegistryIDs []string
}

func ImageCredsWithAWSAuth(lookup func() ImageCreds, logger log.Logger, config AWSRegistryConfig) (func() ImageCreds, error) {
	awsCreds := NoCredentials()
	var credsExpire time.Time

	refresh := func(now time.Time) error {
		var err error
		awsCreds, err = fetchAWSCreds(config)
		if err != nil {
			// bump this along so we don't spam the log
			credsExpire = now.Add(time.Hour)
			return err
		}
		credsExpire = now.Add(tokenValid)
		return nil
	}

	// pre-flight check
	if err := refresh(time.Now()); err != nil {
		return nil, err
	}

	return func() ImageCreds {
		imageCreds := lookup()

		now := time.Now()
		if now.After(credsExpire) {
			if err := refresh(now); err != nil {
				logger.Log("warning", "AWS token not refreshed", "err", err)
			}
		}

		for name, creds := range imageCreds {
			if strings.HasSuffix(name.Domain, ecrHostSuffix) {
				newCreds := NoCredentials()
				newCreds.Merge(awsCreds)
				newCreds.Merge(creds)
				imageCreds[name] = newCreds
			}
		}
		return imageCreds
	}, nil
}

func fetchAWSCreds(config AWSRegistryConfig) (Credentials, error) {
	sess := session.Must(session.NewSession(&aws.Config{Region: &config.Region}))
	svc := ecr.New(sess)
	ecrToken, err := svc.GetAuthorizationToken(&ecr.GetAuthorizationTokenInput{
		RegistryIds: aws.StringSlice(config.RegistryIDs),
	})
	if err != nil {
		return Credentials{}, err
	}
	auths := make(map[string]creds)
	for _, v := range ecrToken.AuthorizationData {
		// Remove the https prefix
		host := strings.TrimPrefix(*v.ProxyEndpoint, "https://")
		creds, err := parseAuth(*v.AuthorizationToken)
		if err != nil {
			return Credentials{}, err
		}
		creds.provenance = "AWS API"
		creds.registry = host
		auths[host] = creds
	}
	return Credentials{m: auths}, nil
}
