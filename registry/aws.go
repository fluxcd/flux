package registry

// References:
//  - https://github.com/bzon/ecr-k8s-secret-creator
//  - https://github.com/kubernetes/kubernetes/blob/master/pkg/credentialprovider/aws/aws_credentials.go
//  - https://github.com/weaveworks/flux/pull/1455

import (
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/go-kit/kit/log"
)

const (
	// For recognising ECR hosts
	ecrHostSuffix = ".amazonaws.com"
	// How long AWS tokens remain valid, according to AWS docs; this
	// is used as an upper bound, overridden by any sooner expiry
	// returned in the API response.
	defaultTokenValid = 12 * time.Hour
	// how long to skip refreshing a region after we've failed
	embargoDuration = 10 * time.Minute

	EKS_SYSTEM_ACCOUNT = "602401143452"
)

type AWSRegistryConfig struct {
	Regions    []string
	AccountIDs []string
	BlockIDs   []string
}

func contains(strs []string, str string) bool {
	for _, s := range strs {
		if s == str {
			return true
		}
	}
	return false
}

// ECR registry URLs look like this:
//
//     <account-id>.dkr.ecr.<region>.amazonaws.com
//
// i.e., they can differ in the account ID and in the region. It's
// possible to refer to any registry from any cluster (although, being
// AWS, there will be a cost incurred).

func ImageCredsWithAWSAuth(lookup func() ImageCreds, logger log.Logger, config AWSRegistryConfig) (func() ImageCreds, error) {
	awsCreds := NoCredentials()

	if len(config.Regions) == 0 {
		// this forces the AWS SDK to load config, so we can get the default region
		sess := session.Must(session.NewSessionWithOptions(session.Options{
			SharedConfigState: session.SharedConfigEnable,
		}))
		clusterRegion := *sess.Config.Region
		if clusterRegion == "" {
			// no region set in config; in that case, use the EC2 metadata service to find where we are running.
			ec2 := ec2metadata.New(sess)
			instanceRegion, err := ec2.Region()
			if err != nil {
				logger.Log("warn", "no AWS region configured, or detected as cluster region", "err", err)
				return nil, err
			}
			clusterRegion = instanceRegion
		}
		logger.Log("info", "detected cluster region", "region", clusterRegion)
		config.Regions = []string{clusterRegion}
	}

	logger.Log("info", "restricting ECR registry scans",
		"regions", strings.Join(config.Regions, ", "),
		"include-ids", strings.Join(config.AccountIDs, ", "),
		"exclude-ids", strings.Join(config.BlockIDs, ", "))

	// this has the expiry time from the last request made per region. We request new tokens whenever
	//  - we don't have credentials for the particular registry URL
	//  - the credentials have expired
	// and when we do, we get new tokens for all account IDs in the
	// region that we've seen. This means that credentials are
	// fetched, and expire, per region.
	regionExpire := map[string]time.Time{}
	// we can get an error when refreshing the credentials; to avoid
	// spamming the log, keep track of failed refreshes.
	regionEmbargo := map[string]time.Time{}

	// should this registry be scanned?
	var shouldScan func(string, string) bool
	if len(config.AccountIDs) == 0 {
		shouldScan = func(region, accountID string) bool {
			return contains(config.Regions, region) && !contains(config.BlockIDs, accountID)
		}
	} else {
		shouldScan = func(region, accountID string) bool {
			return contains(config.Regions, region) &&
				contains(config.AccountIDs, accountID) &&
				!contains(config.BlockIDs, accountID)
		}
	}

	ensureCreds := func(domain, region, accountID string, now time.Time) error {
		// if we had an error getting a token before, don't try again
		// until the embargo has passed
		if embargo, ok := regionEmbargo[region]; ok {
			if embargo.After(now) {
				return nil // i.e., fail silently
			}
			delete(regionEmbargo, region)
		}

		// if we don't have the entry at all, we need to get a
		// token. NB we can't check the inverse and return early,
		// since if the creds do exist, we need to check their expiry.
		if c := awsCreds.credsFor(domain); c == (creds{}) {
			goto refresh
		}

		// otherwise, check if the tokens have expired
		if expiry, ok := regionExpire[region]; !ok || expiry.Before(now) {
			goto refresh
		}

		// the creds exist and are before the use-by; nothing to be done.
		return nil

	refresh:
		// unconditionally append the sought-after account, and let
		// the AWS API figure out if it's a duplicate.
		accountIDs := append(allAccountIDsInRegion(awsCreds.Hosts(), region), accountID)
		logger.Log("info", "attempting to refresh auth tokens", "region", region, "account-ids", strings.Join(accountIDs, ", "))
		regionCreds, expiry, err := fetchAWSCreds(region, accountIDs)
		if err != nil {
			regionEmbargo[region] = now.Add(embargoDuration)
			logger.Log("error", "fetching credentials for AWS region", "region", region, "err", err, "embargo", embargoDuration)
			return err
		}
		regionExpire[region] = expiry
		awsCreds.Merge(regionCreds)
		return nil
	}

	return func() ImageCreds {
		imageCreds := lookup()

		for name, creds := range imageCreds {
			domain := name.Domain
			if strings.HasSuffix(domain, ecrHostSuffix) {
				bits := strings.Split(domain, ".")
				if len(bits) != 6 {
					logger.Log("warning", "AWS registry domain not in expected format <account-id>.dkr.ecr.<region>.amazonaws.com", "domain", domain)
					continue
				}
				accountID := bits[0]
				region := bits[3]

				if !shouldScan(region, accountID) {
					delete(imageCreds, name)
					continue
				}
				if err := ensureCreds(domain, region, accountID, time.Now()); err != nil {
					logger.Log("warning", "unable to ensure credentials for ECR", "domain", domain, "err", err)
				}
				newCreds := NoCredentials()
				newCreds.Merge(awsCreds)
				newCreds.Merge(creds)
				imageCreds[name] = newCreds
			}
		}
		return imageCreds
	}, nil
}

func allAccountIDsInRegion(hosts []string, region string) []string {
	var ids []string
	// this returns a list of unique accountIDs, assuming that the input is unique hostnames
	for _, host := range hosts {
		bits := strings.Split(host, ".")
		if len(bits) != 6 {
			continue
		}
		if bits[3] == region {
			ids = append(ids, bits[0])
		}
	}
	return ids
}

func fetchAWSCreds(region string, accountIDs []string) (Credentials, time.Time, error) {
	sess := session.Must(session.NewSession(&aws.Config{Region: aws.String(region)}))
	svc := ecr.New(sess)
	ecrToken, err := svc.GetAuthorizationToken(&ecr.GetAuthorizationTokenInput{
		RegistryIds: aws.StringSlice(accountIDs),
	})
	if err != nil {
		return Credentials{}, time.Time{}, err
	}
	auths := make(map[string]creds)
	expiry := time.Now().Add(defaultTokenValid)
	for _, v := range ecrToken.AuthorizationData {
		// Remove the https prefix
		host := strings.TrimPrefix(*v.ProxyEndpoint, "https://")
		creds, err := parseAuth(*v.AuthorizationToken)
		if err != nil {
			return Credentials{}, time.Time{}, err
		}
		creds.provenance = "AWS API"
		creds.registry = host
		auths[host] = creds
		ex := *v.ExpiresAt
		if ex.Before(expiry) {
			expiry = ex
		}
	}
	return Credentials{m: auths}, expiry, nil
}
