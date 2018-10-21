package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/go-kit/kit/log"
	"github.com/spf13/pflag"
	fluxecr "github.com/weaveworks/flux/sidecar/aws/ecr"
)

func main() {
	// Flag domain.
	fs := pflag.NewFlagSet("default", pflag.ContinueOnError)
	var (
		awsRegistryIDs = fs.StringSlice("registry-ids", []string{}, "list of AWS ECR account IDs to authenticate")
		awsRegion      = fs.String("region", "us-east-1", "AWS Region to authenticate to")
		fetchInterval  = fs.Duration("fetch-interval", 1*time.Hour, "period at which to fetch for a new authentication token. Authorization tokens are valid for 12 hours")
	)
	fs.Parse(os.Args[1:])

	var logger log.Logger
	{
		logger = log.NewLogfmtLogger(os.Stderr)
		logger = log.With(logger, "ts", log.DefaultTimestampUTC)
		logger = log.With(logger, "caller", log.DefaultCaller)
	}
	logger.Log("sidecar_type", "aws")
	logger.Log("registry_ids", fmt.Sprintf("%s", *awsRegistryIDs), "region", *awsRegion, "fetch_interval", fmt.Sprintf("%dm", *fetchInterval/time.Minute))
	logger.Log("sidecar_uri", "http://localhost:"+fluxecr.SidecarAWSPort+fluxecr.SidecarAWSPath)

	var ecrCredentials []byte
	go func() {
		for {
			sess := session.Must(session.NewSession(&aws.Config{Region: awsRegion}))
			ecrSvc := ecr.New(sess)
			cred, err := fluxecr.GetAmazonECRToken(ecrSvc, *awsRegistryIDs)
			if err != nil {
				logger.Log("error", err)
				os.Exit(1)
			}
			logger.Log("info", "successfully fetched ECR credentials")
			ecrCredentials = []byte(cred.String())
			logger.Log("info", "ECR credentials are updated in memory")
			time.Sleep(*fetchInterval)
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc(fluxecr.SidecarAWSPath, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(ecrCredentials); err != nil {
			logger.Log("err", err)
		}
	})
	if err := http.ListenAndServe(":"+fluxecr.SidecarAWSPort, mux); err != nil {
		logger.Log("err", err)
		os.Exit(1)
	}
}
