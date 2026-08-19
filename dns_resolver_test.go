package main

import (
	"os"
	"testing"
	"time"

	"github.com/cert-manager/cert-manager/test/acme"
)

var (
	zone       = os.Getenv("TEST_ZONE_NAME")
	dnsServer  = os.Getenv("TEST_DNS_SERVER")
	apiVersion = os.Getenv("TEST_API_VERSION")
	testTimeout = os.Getenv("TEST_TIMEOUT")
)

func TestRunsSuite(t *testing.T) {
	pollTime, _ := time.ParseDuration("5s")

	timeoutStr := testTimeout
	if timeoutStr == "" {
		timeoutStr = "3m"
	}
	timeOut, _ := time.ParseDuration(timeoutStr)

	if dnsServer == "" {
		dnsServer = "1.1.1.1:53"
	}

	version := apiVersion
	if version == "" {
		version = "v1"
	}
	manifestPath := "testdata/godaddy/" + version

	t.Logf("Using GoDaddy API version: %s (manifest: %s)", apiVersion, manifestPath)

	fixture := dns.NewFixture(&godaddyDNSSolver{},
		dns.SetResolvedZone(zone),
		dns.SetAllowAmbientCredentials(false),
		dns.SetManifestPath(manifestPath),
		dns.SetDNSServer(dnsServer),
		dns.SetUseAuthoritative(false),

		// Disable the extended test as godaddy do not support to create several records for the same Record DNS Name !!
		dns.SetStrict(false),

		dns.SetPollInterval(pollTime),
		dns.SetPropagationLimit(timeOut),
	)

	fixture.RunConformance(t)
}
