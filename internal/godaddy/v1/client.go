package v1

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/sirupsen/logrus"
	"github.com/snowdrop/godaddy-webhook/internal/godaddy"
)

type client struct {
	cfg *godaddy.ClientConfig
}

func NewClient(cfg *godaddy.ClientConfig) godaddy.Client {
	return &client{cfg: cfg}
}

func setAuth(req *http.Request, cfg *godaddy.ClientConfig) {
	req.Header.Set("Authorization", fmt.Sprintf("sso-key %s:%s", cfg.AuthAPIKey, cfg.AuthAPISecret))
}

func (c *client) HasTXTRecord(domainZone, recordName, challengeKey string) (bool, error) {
	url := fmt.Sprintf("/v1/domains/%s/records/TXT/%s", domainZone, recordName)
	logrus.Debug("### GoDaddy credentials loaded")
	logrus.Infof("### URL request issued to check if the TXT DNS record is present: %s", url)

	resp, err := godaddy.MakeRequest(c.cfg, http.MethodGet, url, nil, setAuth)
	if err != nil {
		logrus.Infof("### HTTP request failed with Godaddy: %s", err)
		return false, err
	}
	defer resp.Body.Close()
	logrus.Debugf("### Godaddy HTTP body response: %s", resp.Body)

	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	} else if resp.StatusCode == http.StatusOK {
		var dnsRecords []godaddy.DNSRecord
		err = json.NewDecoder(resp.Body).Decode(&dnsRecords)
		if err != nil {
			return false, fmt.Errorf("### HTTP response body cannot be parsed to JSON: %s", err)
		}

		if len(dnsRecords) == 0 {
			logrus.Info("### No TXT Record found using godaddy REST API !")
			return false, nil
		}

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

	return false, fmt.Errorf("### Unexpected HTTP status: %d", resp.StatusCode)
}

func (c *client) UpdateRecords(records []godaddy.DNSRecord, domainZone, recordName string) error {
	body, err := json.Marshal(records)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("/v1/domains/%s/records/TXT/%s", domainZone, recordName)
	logrus.Infof("### URL request issued to create/update the DNS record: %s", url)
	logrus.Debugf("### DNS record(s): %s", body)

	resp, err := godaddy.MakeRequest(c.cfg, http.MethodPut, url, bytes.NewReader(body), setAuth)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("### Could not create record %v; Status: %v; Body: %s", string(body), resp.StatusCode, string(bodyBytes))
	}

	logrus.Info("### TXT record created/updated using godaddy REST API !")
	return nil
}

func (c *client) DeleteTxtRecord(domainZone, recordName string) error {
	url := fmt.Sprintf("/v1/domains/%s/records/TXT/%s", domainZone, recordName)
	logrus.Infof("### URL request issued to delete the DNS record: %s", url)

	resp, err := godaddy.MakeRequest(c.cfg, http.MethodDelete, url, nil, setAuth)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("### Failed deleting TXT record: status of the response: %d", resp.StatusCode)
	}

	logrus.Infof("### TXT Record deleted using Godaddy REST API")
	return nil
}