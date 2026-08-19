package godaddy

import (
	"fmt"
	"io"
	"net/http"
	"time"

	useragent "github.com/cert-manager/cert-manager/pkg/util"
	logrus "github.com/sirupsen/logrus"
)

type DNSRecord struct {
	Type     string `json:"type"`
	Name     string `json:"name"`
	Data     string `json:"data"`
	Priority int    `json:"priority,omitempty"`
	TTL      int    `json:"ttl"`
}

type Client interface {
	HasTXTRecord(domainZone, recordName, challengeKey string) (bool, error)
	UpdateRecords(records []DNSRecord, domainZone, recordName string) error
	DeleteTxtRecord(domainZone, recordName string) error
}

type ClientConfig struct {
	AuthAPIKey    string
	AuthAPISecret string
	AuthPAT       string
	Production    bool
	// BaseURL overrides the default API URL. Used for testing.
	BaseURL string
}

func APIBaseURL(production bool) string {
	if production {
		return "https://api.godaddy.com"
	}
	return "https://api.ote-godaddy.com"
}

func MakeRequest(cfg *ClientConfig, method, uri string, body io.Reader, setAuth func(*http.Request, *ClientConfig)) (*http.Response, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = APIBaseURL(cfg.Production)
	}
	req, err := http.NewRequest(method, fmt.Sprintf("%s%s", baseURL, uri), body)
	if err != nil {
		return nil, err
	}

	certManagerUserAgent := "cert-manager/" + useragent.AppVersion

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", certManagerUserAgent)
	setAuth(req, cfg)

	logrus.Debugf("### Godaddy HTTP request: %s", req.URL.String())
	logrus.Debug("### Authorization header set")
	client := http.Client{
		Timeout: 30 * time.Second,
	}

	return client.Do(req)
}