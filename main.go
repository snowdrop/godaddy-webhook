package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cert-manager/cert-manager/pkg/acme/webhook/cmd"
	"github.com/cert-manager/cert-manager/pkg/issuer/acme/dns/util"
	useragent "github.com/cert-manager/cert-manager/pkg/util"
	"github.com/snowdrop/godaddy-webhook/logging"

	logrus "github.com/sirupsen/logrus"
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	LOGGING_LEVEL_ENV_NAME     = "LOGGING_LEVEL"
	LOGGING_FORMAT_ENV_NAME    = "LOGGING_FORMAT"
	LOGGING_TIMESTAMP_ENV_NAME = "LOGGING_TIMESTAMP"
)

var (
	logLevel        = os.Getenv(LOGGING_LEVEL_ENV_NAME)     // Log level (trace, debug, info, warn, error, fatal, panic)
	logFormat       = os.Getenv(LOGGING_FORMAT_ENV_NAME)    // Log format (text, color, json)
	logTimestampStr = os.Getenv(LOGGING_TIMESTAMP_ENV_NAME) // Timestamp in log output
	logTimestamp    bool
	GroupName       = os.Getenv("GROUP_NAME")
)

// DNSRecord a DNS record
type DNSRecord struct {
	Type     string `json:"type"`
	Name     string `json:"name"`
	Data     string `json:"data"`
	Priority int    `json:"priority,omitempty"`
	TTL      int    `json:"ttl,omitempty"`
}

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

	// This will register our godaddy DNS provider with the webhook serving
	// library, making it available as an API under the provided GroupName.
	// You can register multiple DNS provider implementations with a single
	// webhook, where the Name() method will be used to disambiguate between
	// the different implementations.
	cmd.RunWebhookServer(GroupName,
		&godaddyDNSSolver{},
	)
}

// godaddyDNSSolver implements the provider-specific logic needed to
// 'present' an ACME challenge TXT record for your own DNS provider.
// To do so, it must implement the `github.com/cert-manager/cert-manager/pkg/acme/webhook.Solver`
// interface.
type godaddyDNSSolver struct {
	client *kubernetes.Clientset
	cfg    *godaddyDNSProviderConfig
}

// godaddyDNSProviderConfig is a structure that is used to decode into when
// solving a DNS01 challenge.
// This information is provided by cert-manager, and may be a reference to
// additional configuration that's needed to solve the challenge for this
// particular certificate or issuer.
// This typically includes references to Secret resources containing DNS
// provider credentials, in cases where a 'multi-tenant' DNS solver is being
// created.
// If you do *not* require per-issuer or per-certificate configuration to be
// provided to your webhook, you can skip decoding altogether in favour of
// using CLI flags or similar to provide configuration.
// You should not include sensitive information here. If credentials need to
// be used by your provider here, you should reference a Kubernetes Secret
// resource and fetch these credentials using a Kubernetes clientset.
type godaddyDNSProviderConfig struct {
	// These fields will be set by users in the
	// `issuer.spec.acme.dns01.providers.webhook.config` field.

	APIKeySecretRef certmgrv1.SecretKeySelector `json:"apiKeySecretRef"`

	AuthAPIKey    string `json:"authApiKey"`
	AuthAPISecret string `json:"authApiSecret"`
	Production    bool   `json:"production"`

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
	// Try to load the API key
	if cfg.APIKeySecretRef.LocalObjectReference.Name == "" {
		return errors.New("API token field were not provided as no Kubernetes Secret exists !")
	}
	return nil
}

// Name is used as the name for this DNS solver when referencing it on the ACME
// Issuer resource.
// This should be unique **within the group name**, i.e. you can have two
// solvers configured with the same Name() **so long as they do not co-exist
// within a single webhook deployment**.
// For example, `cloudflare` may be used as the name of a solver.
func (c *godaddyDNSSolver) Name() string {
	return providerName
}

// Return GoDaddi API URL to query the API domains
// See - https://developer.godaddy.com/doc/endpoint/domains
// OTE environment: https://api.ote-godaddy.com
// PRODUCTION environment: https://api.godaddy.com
func (c *godaddyDNSSolver) apiURL(cfg *godaddyDNSProviderConfig) string {
	baseURL := "https://api.ote-godaddy.com"
	if cfg.Production {
		baseURL = "https://api.godaddy.com"
	}
	return baseURL
}

func (c *godaddyDNSSolver) extractApiTokenFromSecret(cfg *godaddyDNSProviderConfig, ch *v1alpha1.ChallengeRequest) error {
	sec, err := c.client.CoreV1().
		Secrets(ch.ResourceNamespace).
		Get(context.TODO(), cfg.APIKeySecretRef.LocalObjectReference.Name, metaV1.GetOptions{})
	if err != nil {
		return err
	}

	secBytes, ok := sec.Data[cfg.APIKeySecretRef.Key]
	if !ok {
		return fmt.Errorf("Key %q not found in secret \"%s/%s\"",
			cfg.APIKeySecretRef.Key,
			cfg.APIKeySecretRef.LocalObjectReference.Name,
			ch.ResourceNamespace)
	}

	token := strings.Split(string(secBytes), ":")
	cfg.AuthAPIKey = token[0]
	cfg.AuthAPISecret = token[1]

	return nil
}

// Present is responsible for actually presenting the DNS record with the
// DNS provider.
// This method should tolerate being called multiple times with the same value.
// cert-manager itself will later perform a self check to ensure that the
// solver has correctly configured the DNS provider.
func (c *godaddyDNSSolver) Present(ch *v1alpha1.ChallengeRequest) error {
	cfg, err := loadConfig(ch.Config)
	if err != nil {
		return err
	}

	// Verify if the config contains the required parameters such as SecretRef
	if err := c.validate(cfg); err != nil {
		return err
	}

	// Extract the Godaddy Api and Secret from the K8s Secret
	// and assign it the AuthAPIKey and AuthAPISecret of the Config
	if err := c.extractApiTokenFromSecret(cfg, ch); err != nil {
		return err
	}

	recordName := c.extractRecordName(ch.ResolvedFQDN, ch.ResolvedZone)
	logrus.Infof("TXT Record name: %s", recordName)

	dnsZone, err := c.getZone(ch.ResolvedZone)
	if err != nil {
		return err
	}

	logrus.Infof("### Try to present the DNS record with the DNS provider using as challengeKey: %s", ch.Key)
	_, err = c.HasTXTRecord(cfg, dnsZone, recordName, ch.Key)
	if err != nil {
		return fmt.Errorf("Unable to check the TXT record: %v", err)
	}

	rec := []DNSRecord{{
		Data: c.TXTRecordContent(ch.Key),
		TTL:  cfg.TTL,
		Type: "TXT",
		Name: recordName,
	},
	}

	err = c.UpdateRecords(cfg, rec, dnsZone, recordName)
	if err != nil {
		return fmt.Errorf("### Unable to create TXT record: %v", err)
	}

	return nil
}

func (c *godaddyDNSSolver) TXTRecordContent(key string) string {
	if key != "" {
		return key
	} else {
		return "null"
	}
}

// CleanUp should delete the relevant TXT record from the DNS provider console.
// If multiple TXT records exist with the same record name (e.g.
// _acme-challenge.example.com) then **only** the record with the same `key`
// value provided on the ChallengeRequest should be cleaned up.
// This is in order to facilitate multiple DNS validations for the same domain
// concurrently.
func (c *godaddyDNSSolver) CleanUp(ch *v1alpha1.ChallengeRequest) error {
	cfg, err := loadConfig(ch.Config)
	if err != nil {
		return err
	}

	// Verify if the config contains the required parameters such as SecretRef
	if err := c.validate(cfg); err != nil {
		return err
	}

	// Extract the Godaddy Api and Secret from the K8s Secret
	// and assign it the AuthAPIKey and AuthAPISecret of the Config
	if err := c.extractApiTokenFromSecret(cfg, ch); err != nil {
		return err
	}

	recordName := c.extractRecordName(ch.ResolvedFQDN, ch.ResolvedZone)
	dnsZone, err := c.getZone(ch.ResolvedZone)
	if err != nil {
		return err
	}

	logrus.Infof("### CleanUp should delete the relevant TXT record for the challengeKey: %s", ch.Key)
	present, err := c.HasTXTRecord(cfg, dnsZone, recordName, ch.Key)
	if err != nil {
		return fmt.Errorf("### Unable to check TXT record: %s", err)
	}

	if present {
		logrus.Infof("### Deleting entry=%s, domain=%s", recordName, dnsZone)
		err := c.DeleteTxtRecord(cfg, dnsZone, recordName)
		if err != nil {
			return fmt.Errorf("### Unable to delete the TXT record: %v", err)
		}
	}

	return nil
}

// Initialize will be called when the webhook first starts.
// This method can be used to instantiate the webhook, i.e. initialising
// connections or warming up caches.
// Typically, the kubeClientConfig parameter is used to build a Kubernetes
// client that can be used to fetch resources from the Kubernetes API, e.g.
// Secret resources containing credentials used to authenticate with DNS
// provider accounts.
// The stopCh can be used to handle early termination of the webhook, in cases
// where a SIGTERM or similar signal is sent to the webhook process.
func (c *godaddyDNSSolver) Initialize(kubeClientConfig *rest.Config, stopCh <-chan struct{}) error {
	cl, err := kubernetes.NewForConfig(kubeClientConfig)
	if err != nil {
		return err
	}

	c.client = cl
	return nil
}

// loadConfig is a small helper function that decodes JSON configuration into
// the typed config struct.
func loadConfig(cfgJSON *apiext.JSON) (*godaddyDNSProviderConfig, error) {
	cfg := &godaddyDNSProviderConfig{}
	// handle the 'base case' where no configuration has been provided
	if cfgJSON == nil {
		return cfg, nil
	}
	if err := json.Unmarshal(cfgJSON.Raw, &cfg); err != nil {
		return cfg, fmt.Errorf("error decoding solver config: %v", err)
	}

	return cfg, nil
}

func (c *godaddyDNSSolver) HasTXTRecord(cfg *godaddyDNSProviderConfig, domainZone string, recordName string, challengeKey string) (bool, error) {
	// curl -X GET -H "Authorization: sso-key $TOKEN"
	// "https://api.godaddy.com/v1/domains/<DOMAIN>/records/TXT/<NAME>"
	url := fmt.Sprintf("/v1/domains/%s/records/TXT/%s", domainZone, recordName)
	logrus.Debugf("### Godaddy Api: %s, Secret: %s keys", cfg.AuthAPIKey, cfg.AuthAPISecret)
	logrus.Infof("### URL request issued to check if the TXT DNS record is present: %s", url)

	resp, err := c.makeRequest(cfg, http.MethodGet, url, nil)
	if err != nil {
		logrus.Infof("### HTTP request failed with Godaddy: %s", err)
		return false, err
	}
	logrus.Debugf("### Godaddy HTTP body response: %s", resp.Body)

	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	} else if resp.StatusCode == http.StatusOK {
		var dnsRecords = []DNSRecord{}
		err = json.NewDecoder(resp.Body).Decode(&dnsRecords)
		if err != nil {
			return false, fmt.Errorf("### HTTP response body cannot be parsed to JSON: %s", err)
		}

		if len(dnsRecords) == 0 {
			logrus.Info("### No TXT Record found using godaddy REST API !")
			return false, nil
		} else {
			for _, dnsRecord := range dnsRecords {
				logrus.Infof("### TXT Record collected from godaddy: %#v", dnsRecord)
				if dnsRecord.Data == challengeKey {
					logrus.Infof("### TXT Record found : %#v, for challengeKey: %s", dnsRecord, challengeKey)
					return true, nil
				}
			}
			logrus.Infof("### No TXT Record found within the response for challengeKey: %s", challengeKey)
			return false, nil
		}
	} else {
		return false, fmt.Errorf("### Unexpected HTTP status: %d", resp.StatusCode)
	}

	return false, nil
}

// Function to be used to create/update a TXT record
// Godaddy uses an array of DNS records as input !
// See: https://developer.godaddy.com/doc/endpoint/domains#/v1/recordReplaceType
func (c *godaddyDNSSolver) UpdateRecords(cfg *godaddyDNSProviderConfig, records []DNSRecord, domainZone string, recordName string) error {
	body, err := json.Marshal(records)
	if err != nil {
		return err
	}

	var resp *http.Response
	url := fmt.Sprintf("/v1/domains/%s/records/TXT/%s", domainZone, recordName)
	logrus.Infof("### URL request issued to create/update the DNS record: %s", url)
	logrus.Debugf("### DNS record(s): %s", body)
	resp, err = c.makeRequest(cfg, http.MethodPut, url, bytes.NewReader(body))
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("### Could not create record %v; Status: %v; Body: %s", string(body), resp.StatusCode, string(bodyBytes))
	} else {
		logrus.Info("### TXT record created/updated using godaddy REST API !")
	}
	return nil
}

// Function to be used to delete a TXT record
// See: https://developer.godaddy.com/doc/endpoint/domains#/v1/recordDeleteTypeName
func (c *godaddyDNSSolver) DeleteTxtRecord(cfg *godaddyDNSProviderConfig, domainZone string, recordName string) error {
	var resp *http.Response
	url := fmt.Sprintf("/v1/domains/%s/records/TXT/%s", domainZone, recordName)
	logrus.Infof("### URL request issued to delete the DNS record: %s", url)

	resp, err := c.makeRequest(cfg, http.MethodDelete, url, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("### Failed deleting TXT record: %v, status of the response: %d", err, resp.StatusCode)
	}
	logrus.Infof("### TXT Record deleted using Godaddy REST API")
	return nil
}

func (c *godaddyDNSSolver) makeRequest(cfg *godaddyDNSProviderConfig, method string, uri string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, fmt.Sprintf("%s%s", c.apiURL(cfg), uri), body)
	if err != nil {
		return nil, err
	}

	var CertManagerUserAgent = "cert-manager/" + useragent.AppVersion

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", CertManagerUserAgent)
	req.Header.Set("Authorization", fmt.Sprintf("sso-key %s:%s", cfg.AuthAPIKey, cfg.AuthAPISecret))

	logrus.Debugf("### Godaddy HTTP request: %s", req.URL.String())
	logrus.Debugf("### Header authorisation: %s", req.Header.Get("Authorization"))

	client := http.Client{
		Timeout: 30 * time.Second,
	}

	return client.Do(req)
}

func (c *godaddyDNSSolver) extractRecordName(fqdn, domain string) string {
	if idx := strings.Index(fqdn, "."+domain); idx != -1 {
		return fqdn[:idx]
	}
	return util.UnFqdn(fqdn)
}

func (c *godaddyDNSSolver) extractDomainName(zone string) string {
	authZone, err := util.FindZoneByFqdn(context.TODO(), zone, util.RecursiveNameservers)
	if err != nil {
		return zone
	}
	return util.UnFqdn(authZone)
}

func (c *godaddyDNSSolver) getZone(fqdn string) (string, error) {
	authZone, err := util.FindZoneByFqdn(context.TODO(), fqdn, util.RecursiveNameservers)
	if err != nil {
		return "", err
	}

	return util.UnFqdn(authZone), nil
}

func (c *godaddyDNSSolver) getDomainAndEntry(ch *v1alpha1.ChallengeRequest) (string, string) {
	// Both ch.ResolvedZone and ch.ResolvedFQDN end with a dot: '.'
	entry := strings.TrimSuffix(ch.ResolvedFQDN, ch.ResolvedZone)
	entry = strings.TrimSuffix(entry, ".")
	domain := strings.TrimSuffix(ch.ResolvedZone, ".")
	return entry, domain
}
