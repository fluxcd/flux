package registry

import (
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecr"
	"go.uber.org/zap"
)

const (
	// For recognising ECR hosts
	awsPartitionSuffix   = ".amazonaws.com"
	awsCnPartitionSuffix = ".amazonaws.com.cn"

	// How long AWS tokens remain valid, according to AWS docs; this
	// is used as an upper bound, overridden by any sooner expiry
	// returned in the API response.
	defaultTokenValid = 12 * time.Hour
	// how long to skip refreshing a region after we've failed
	embargoDuration = 10 * time.Minute

	EKS_SYSTEM_ACCOUNT    = "602401143452"
	EKS_SYSTEM_ACCOUNT_CN = "918309763551"
)

// AWSRegistryConfig supplies constraints for scanning AWS (ECR) image
// registries. Fields may be left empty.
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

func validECRHost(domain string) bool {
	switch {
	case strings.HasSuffix(domain, awsPartitionSuffix):
		return true
	case strings.HasSuffix(domain, awsCnPartitionSuffix):
		return true
	}
	return false
}

// ImageCredsWithAWSAuth wraps an image credentials func with another
// that adds two capabilities:
//
//   - it will include or exclude images from ECR accounts and regions
//     according to the config given; and,
//
//   - if it can reach the AWS API, it will obtain credentials for ECR
//     accounts from it, automatically refreshing them when necessary.
//
// It also returns a "pre-flight check" that can be used to verify
// that the AWS API is available while starting up.
//
// ECR registry URLs look like this:
//
//     <account-id>.dkr.ecr.<region>.amazonaws.com
//
// i.e., they can differ in the account ID and in the region. It's
// possible to refer to any registry from any cluster (although, being
// AWS, there will be a cost incurred). The config supplied can
// restrict based on the region:
//
//  - if a region or regions are supplied, exactly those regions shall
//    be included;
//  - if no region is supplied, but it can be detected, the detected
//    region is included
//  - if no region is supplied _or_ detected, no region is included
//
//  .. and on the account ID:
//
//  - if account IDs to include are supplied, only those are included
//    - otherwise, all account IDs are included
//    - the supplied list may be empty
//  with the exception
//  - if account IDs to _exclude_ are supplied, those shall be not be
//    included
func ImageCredsWithAWSAuth(lookup func() ImageCreds, logger *zap.Logger, config AWSRegistryConfig) (func() error, func() ImageCreds) {
	// only ever do the preflight check once; all subsequent calls
	// will succeed trivially, so the first caller should pay
	// attention to the return value.
	var preflightOnce sync.Once
	// it's possible to fail the pre-flight check, but still apply the
	// constraints given in the config. `okToUseAWS` is true if using
	// the AWS API to get credentials is expected to work.
	var okToUseAWS bool

	preflight := func() error {
		var preflightErr error
		preflightOnce.Do(func() {

			defer func() {
				logger.Info(
					"restricting ECR registry scans",
					zap.Strings("regions", config.Regions),
					zap.Strings("include-ids", config.AccountIDs),
					zap.Strings("exclude-ids", config.BlockIDs),
				)
			}()

			// This forces the AWS SDK to load config, so we can get
			// the default region if it's there.
			sess := session.Must(session.NewSessionWithOptions(session.Options{
				SharedConfigState: session.SharedConfigEnable,
			}))
			// Always try to connect to the metadata service, so we
			// can fail fast if it's not available.
			ec2 := ec2metadata.New(sess)
			metadataRegion, err := ec2.Region()
			if err != nil {
				preflightErr = err
				if config.Regions == nil {
					config.Regions = []string{}
				}
				logger.Error("fetching region for AWS", zap.NamedError("err", err))
				return
			}

			okToUseAWS = true

			if config.Regions == nil {
				clusterRegion := *sess.Config.Region
				regionSource := "local config"
				if clusterRegion == "" {
					// no region set in config; in that case, use what we got from the EC2 metadata service
					clusterRegion = metadataRegion
					regionSource = "EC2 metadata service"
				}
				logger.Info(
					"detected cluster region",
					zap.String("source", regionSource),
					zap.String("region", clusterRegion),
				)
				config.Regions = []string{clusterRegion}
			}
		})
		return preflightErr
	}

	awsCreds := NoCredentials()

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
	if config.AccountIDs == nil {
		shouldScan = func(region, accountID string) bool {
			return contains(config.Regions, region) && !contains(config.BlockIDs, accountID)
		}
	} else {
		shouldScan = func(region, accountID string) bool {
			return contains(config.Regions, region) && contains(config.AccountIDs, accountID) && !contains(config.BlockIDs, accountID)
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
		logger.Info(
			"attempting to refresh auth tokens",
			zap.String("region", region),
			zap.Strings("account-ids", accountIDs),
		)
		regionCreds, expiry, err := fetchAWSCreds(region, accountIDs)
		if err != nil {
			regionEmbargo[region] = now.Add(embargoDuration)
			logger.Error(
				"fetching credentials for AWS region",
				zap.String("region", region),
				zap.NamedError("err", err),
				zap.Duration("embargo", embargoDuration),
			)
			return err
		}
		regionExpire[region] = expiry
		awsCreds.Merge(regionCreds)
		return nil
	}

	lookupECR := func() ImageCreds {
		imageCreds := lookup()

		for name, creds := range imageCreds {
			domain := name.Domain
			if !validECRHost(domain) {
				continue
			}

			bits := strings.Split(domain, ".")
			if bits[1] != "dkr" || bits[2] != "ecr" {
				logger.Warn("AWS registry domain not in expected format <account-id>.dkr.ecr.<region>.amazonaws.<extension>", zap.String("domain", domain))
				continue
			}
			accountID := bits[0]
			region := bits[3]

			// Before deciding whether an image is included, we need to establish the included regions,
			// and whether we can use the AWS API to get credentials. But we don't need to log any problem
			// that arises _unless_ there's an image that ends up being included in the scanning.
			preflightErr := preflight()

			if !shouldScan(region, accountID) {
				delete(imageCreds, name)
				continue
			}

			if preflightErr != nil {
				logger.Warn("AWS auth implied by ECR image, but AWS API is not available. You can ignore this if you are providing credentials some other way (e.g., through imagePullSecrets)", zap.String("image", name.String()), zap.Error(preflightErr))
			}

			if okToUseAWS {
				if err := ensureCreds(domain, region, accountID, time.Now()); err != nil {
					logger.Warn("unable to ensure credentials for ECR", zap.String("domain", domain), zap.NamedError("err", err))
				}
				newCreds := NoCredentials()
				newCreds.Merge(awsCreds)
				newCreds.Merge(creds)
				imageCreds[name] = newCreds
			}
		}
		return imageCreds
	}

	return preflight, lookupECR
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
