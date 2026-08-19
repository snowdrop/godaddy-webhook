package v3

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/sirupsen/logrus"
	"github.com/snowdrop/godaddy-webhook/internal/godaddy"
)

type dnsRecordResponse struct {
	RecordID string `json:"recordId"`
	Type     string `json:"type"`
	Name     string `json:"name"`
	Data     string `json:"data"`
	TTL      int    `json:"ttl,omitempty"`
}

type dnsRecordsListResponse struct {
	Items []dnsRecordResponse `json:"items"`
}

type client struct {
	cfg *godaddy.ClientConfig
}

func NewClient(cfg *godaddy.ClientConfig) godaddy.Client {
	return &client{cfg: cfg}
}

func setAuth(req *http.Request, cfg *godaddy.ClientConfig) {
	req.Header.Set("Authorization", "Bearer "+cfg.AuthPAT)
}

func (c *client) HasTXTRecord(domainZone, recordName, challengeKey string) (bool, error) {
	url := fmt.Sprintf("/v3/domains/zones/%s/dns-records?type=TXT&name=%s", domainZone, recordName)
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
		var listResp dnsRecordsListResponse
		err = json.NewDecoder(resp.Body).Decode(&listResp)
		if err != nil {
			return false, fmt.Errorf("### HTTP response body cannot be parsed to JSON: %s", err)
		}

		if len(listResp.Items) == 0 {
			logrus.Info("### No TXT Record found using godaddy REST API !")
			return false, nil
		}

		for _, dnsRecord := range listResp.Items {
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
	for _, record := range records {
		if record.TTL == 0 {
			record.TTL = 600
		}
		body, err := json.Marshal(record)
		if err != nil {
			return err
		}

		url := fmt.Sprintf("/v3/domains/zones/%s/dns-records", domainZone)
		logrus.Infof("### URL request issued to create the DNS record: %s", url)
		logrus.Debugf("### DNS record: %s", body)

		resp, err := godaddy.MakeRequest(c.cfg, http.MethodPost, url, bytes.NewReader(body), setAuth)
		if err != nil {
			return err
		}
		if err := func() error {
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusUnprocessableEntity {
				bodyBytes, _ := io.ReadAll(resp.Body)
				if bytes.Contains(bodyBytes, []byte("DUPLICATE_RECORD")) {
					logrus.Infof("### TXT record already exists, skipping: %s", recordName)
					return nil
				}
				return fmt.Errorf("### Could not create record %v; Status: %v; Body: %s", string(body), resp.StatusCode, string(bodyBytes))
			}

			if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
				bodyBytes, _ := io.ReadAll(resp.Body)
				return fmt.Errorf("### Could not create record %v; Status: %v; Body: %s", string(body), resp.StatusCode, string(bodyBytes))
			}

			logrus.Info("### TXT record created using godaddy v3 REST API !")
			return nil
		}(); err != nil {
			return err
		}
	}
	return nil
}

func (c *client) DeleteTxtRecord(domainZone, recordName string) error {
	url := fmt.Sprintf("/v3/domains/zones/%s/dns-records?type=TXT&name=%s", domainZone, recordName)
	logrus.Infof("### URL request issued to list TXT records for deletion: %s", url)

	resp, err := godaddy.MakeRequest(c.cfg, http.MethodGet, url, nil, setAuth)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("### Failed listing TXT records for deletion: status %d", resp.StatusCode)
	}

	var listResp dnsRecordsListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return fmt.Errorf("### Failed to parse DNS records response: %s", err)
	}

	for _, record := range listResp.Items {
		deleteURL := fmt.Sprintf("/v3/domains/zones/%s/dns-records/%s", domainZone, record.RecordID)
		logrus.Infof("### URL request issued to delete DNS record ID %s: %s", record.RecordID, deleteURL)

		delResp, err := godaddy.MakeRequest(c.cfg, http.MethodDelete, deleteURL, nil, setAuth)
		if err != nil {
			return err
		}
		if err := func() error {
			defer delResp.Body.Close()
			if delResp.StatusCode != http.StatusOK && delResp.StatusCode != http.StatusNoContent {
				return fmt.Errorf("### Failed deleting TXT record ID %s: status %d", record.RecordID, delResp.StatusCode)
			}
			logrus.Infof("### TXT Record ID %s deleted using Godaddy v3 REST API", record.RecordID)
			return nil
		}(); err != nil {
			return err
		}
	}

	return nil
}