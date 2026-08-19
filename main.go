package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/cert-manager/cert-manager/pkg/acme/webhook/cmd"
	"github.com/snowdrop/godaddy-webhook/internal/auth"
	"github.com/snowdrop/godaddy-webhook/internal/dns"
	"github.com/snowdrop/godaddy-webhook/internal/godaddy"
	v1 "github.com/snowdrop/godaddy-webhook/internal/godaddy/v1"
	v3 "github.com/snowdrop/godaddy-webhook/internal/godaddy/v3"
	"github.com/snowdrop/godaddy-webhook/internal/logging"

	logrus "github.com/sirupsen/logrus"
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/cert-manager/cert-manager/pkg/acme/webhook/apis/acme/v1alpha1"
	certmgrv1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
)

const (
	providerName        = "godaddy"
	DefaultLevel        = "info"
	DefaultLogTimestamp = false
	DefaultLogFormat    = "color"
	DefaultAPIVersion   = "v1"

	LOGGING_LEVEL_ENV_NAME     = "LOGGING_LEVEL"
	LOGGING_FORMAT_ENV_NAME    = "LOGGING_FORMAT"
	LOGGING_TIMESTAMP_ENV_NAME = "LOGGING_TIMESTAMP"
)

var (
	logLevel        = os.Getenv(LOGGING_LEVEL_ENV_NAME)
	logFormat       = os.Getenv(LOGGING_FORMAT_ENV_NAME)
	logTimestampStr = os.Getenv(LOGGING_TIMESTAMP_ENV_NAME)
	logTimestamp    bool
	GroupName       = os.Getenv("GROUP_NAME")
)

func main() {
	if GroupName == "" {
		panic("GROUP_NAME must be specified")
	}

	if logLevel == "" {
		logLevel = DefaultLevel
	}

	if logFormat == "" {
		logFormat = DefaultLogFormat
	}

	if logTimestampStr == "" {
		logTimestamp = DefaultLogTimestamp
	} else {
		v, err := strconv.ParseBool(logTimestampStr)
		if err != nil {
			logrus.Fatalf("logTimestamp bool assignment failed %s", err)
		} else {
			logTimestamp = v
		}
	}

	if err := logging.Configure(logLevel, logFormat, logTimestamp); err != nil {
		panic(err)
	}

	cmd.RunWebhookServer(GroupName,
		&godaddyDNSSolver{},
	)
}

type godaddyDNSSolver struct {
	client *kubernetes.Clientset
}

type godaddyDNSProviderConfig struct {
	APIKeySecretRef certmgrv1.SecretKeySelector `json:"apiKeySecretRef"`

	AuthAPIKey    string `json:"authApiKey"`
	AuthAPISecret string `json:"authApiSecret"`
	AuthPAT       string `json:"authPAT"`
	Production    bool   `json:"production"`
	APIVersion    string `json:"apiVersion"`

	// +optional. The TTL of the TXT record used for the DNS challenge
	TTL int `json:"ttl"`
	// +optional.  API request timeout
	HttpTimeout int `json:"timeout"`
	// +optional.  Maximum waiting time for DNS propagation
	PropagationTimeout int `json:"propagationTimeout"`
	// +optional. Time between DNS propagation check
	PollingInterval int `json:"pollingInterval"`
	// +optional. Interval between iteration
	SequenceInterval int `json:"sequenceInterval"`
}

func (c *godaddyDNSSolver) validate(cfg *godaddyDNSProviderConfig) error {
	if cfg.APIKeySecretRef.LocalObjectReference.Name == "" {
		return errors.New("apiKeySecretRef.name must be set")
	}
	if cfg.APIKeySecretRef.Key == "" {
		return errors.New("apiKeySecretRef.key must be set")
	}
	switch cfg.APIVersion {
	case "", "v1", "v3":
		return nil
	default:
		return fmt.Errorf("apiVersion must be one of: v1, v3")
	}
}

func (c *godaddyDNSSolver) Name() string {
	return providerName
}

func (c *godaddyDNSSolver) newGodaddyClient(cfg *godaddyDNSProviderConfig) godaddy.Client {
	clientCfg := &godaddy.ClientConfig{
		AuthAPIKey:    cfg.AuthAPIKey,
		AuthAPISecret: cfg.AuthAPISecret,
		AuthPAT:       cfg.AuthPAT,
		Production:    cfg.Production,
	}

	apiVersion := cfg.APIVersion
	if apiVersion == "" {
		apiVersion = DefaultAPIVersion
	}

	logrus.Infof("### Using GoDaddy API version: %s", apiVersion)

	switch apiVersion {
	case "v3":
		return v3.NewClient(clientCfg)
	default:
		return v1.NewClient(clientCfg)
	}
}

func (c *godaddyDNSSolver) extractApiTokenFromSecret(cfg *godaddyDNSProviderConfig, ch *v1alpha1.ChallengeRequest) error {
	if cfg.APIVersion == "v3" {
		pat, err := auth.ExtractPATFromSecret(
			c.client,
			ch.ResourceNamespace,
			cfg.APIKeySecretRef.LocalObjectReference.Name,
			cfg.APIKeySecretRef.Key,
		)
		if err != nil {
			return err
		}
		cfg.AuthPAT = pat
		return nil
	}

	creds, err := auth.ExtractFromSecret(
		c.client,
		ch.ResourceNamespace,
		cfg.APIKeySecretRef.LocalObjectReference.Name,
		cfg.APIKeySecretRef.Key,
	)
	if err != nil {
		return err
	}
	cfg.AuthAPIKey = creds.APIKey
	cfg.AuthAPISecret = creds.APISecret
	return nil
}

func (c *godaddyDNSSolver) Present(ch *v1alpha1.ChallengeRequest) error {
	cfg, err := loadConfig(ch.Config)
	if err != nil {
		return err
	}

	if err := c.validate(cfg); err != nil {
		return err
	}

	if err := c.extractApiTokenFromSecret(cfg, ch); err != nil {
		return err
	}

	recordName := dns.ExtractRecordName(ch.ResolvedFQDN, ch.ResolvedZone)
	logrus.Infof("TXT Record name: %s", recordName)

	dnsZone, err := dns.GetZone(ch.ResolvedZone)
	if err != nil {
		return err
	}

	apiClient := c.newGodaddyClient(cfg)

	logrus.Infof("### Try to present the DNS record with the DNS provider using as challengeKey: %s", ch.Key)
	present, err := apiClient.HasTXTRecord(dnsZone, recordName, ch.Key)
	if err != nil {
		return fmt.Errorf("Unable to check the TXT record: %v", err)
	}

	if present {
		logrus.Infof("### TXT record already exists for challengeKey: %s, skipping create", ch.Key)
		return nil
	}

	rec := []godaddy.DNSRecord{{
		Data: txtRecordContent(ch.Key),
		TTL:  cfg.TTL,
		Type: "TXT",
		Name: recordName,
	}}

	err = apiClient.UpdateRecords(rec, dnsZone, recordName)
	if err != nil {
		return fmt.Errorf("### Unable to create TXT record: %v", err)
	}

	return nil
}

func txtRecordContent(key string) string {
	if key != "" {
		return key
	}
	return "null"
}

func (c *godaddyDNSSolver) CleanUp(ch *v1alpha1.ChallengeRequest) error {
	cfg, err := loadConfig(ch.Config)
	if err != nil {
		return err
	}

	if err := c.validate(cfg); err != nil {
		return err
	}

	if err := c.extractApiTokenFromSecret(cfg, ch); err != nil {
		return err
	}

	recordName := dns.ExtractRecordName(ch.ResolvedFQDN, ch.ResolvedZone)
	dnsZone, err := dns.GetZone(ch.ResolvedZone)
	if err != nil {
		return err
	}

	apiClient := c.newGodaddyClient(cfg)

	logrus.Infof("### CleanUp should delete the relevant TXT record for the challengeKey: %s", ch.Key)
	present, err := apiClient.HasTXTRecord(dnsZone, recordName, ch.Key)
	if err != nil {
		return fmt.Errorf("### Unable to check TXT record: %s", err)
	}

	if present {
		logrus.Infof("### Deleting entry=%s, domain=%s", recordName, dnsZone)
		err := apiClient.DeleteTxtRecord(dnsZone, recordName)
		if err != nil {
			return fmt.Errorf("### Unable to delete the TXT record: %v", err)
		}
	}

	return nil
}

func (c *godaddyDNSSolver) Initialize(kubeClientConfig *rest.Config, stopCh <-chan struct{}) error {
	cl, err := kubernetes.NewForConfig(kubeClientConfig)
	if err != nil {
		return err
	}

	c.client = cl
	return nil
}

func loadConfig(cfgJSON *apiext.JSON) (*godaddyDNSProviderConfig, error) {
	cfg := &godaddyDNSProviderConfig{}
	if cfgJSON == nil {
		return cfg, nil
	}
	if err := json.Unmarshal(cfgJSON.Raw, &cfg); err != nil {
		return cfg, fmt.Errorf("error decoding solver config: %v", err)
	}

	return cfg, nil
}
