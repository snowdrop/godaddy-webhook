package main

import (
	"os"
	"testing"
	"time"

	"github.com/cert-manager/cert-manager/test/acme"
)

var (
	zone      = os.Getenv("TEST_ZONE_NAME")
	dnsServer = os.Getenv("TEST_DNS_SERVER")
)

func TestRunsSuite(t *testing.T) {
	// The manifest path should contain a file named config.json that is a
	// snippet of valid configuration that should be included on the
	// ChallengeRequest passed as part of the test cases.

	pollTime, _ := time.ParseDuration("5s")
	timeOut, _ := time.ParseDuration("3m")

	if dnsServer == "" {
		dnsServer = "1.1.1.1:53"
	}

	fixture := dns.NewFixture(&godaddyDNSSolver{},
		dns.SetResolvedZone(zone),
		dns.SetAllowAmbientCredentials(false),
		dns.SetManifestPath("testdata/godaddy"),
		dns.SetDNSServer(dnsServer),
		dns.SetUseAuthoritative(false),

		// Disable the extended test as godaddy do not support to create several records for the same Record DNS Name !!
		dns.SetStrict(false),

		// Increase the poll interval to 10s
		dns.SetPollInterval(pollTime),
		// Increase the limit from 2 min to 5 min
		dns.SetPropagationLimit(timeOut),
	)

	fixture.RunConformance(t)
}
