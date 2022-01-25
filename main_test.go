package main

import (
	"os"
	"testing"
	"time"

	"github.com/jetstack/cert-manager/test/acme/dns"
)

var (
	zone = os.Getenv("TEST_ZONE_NAME")
)

func TestRunsSuite(t *testing.T) {
	// The manifest path should contain a file named config.json that is a
	// snippet of valid configuration that should be included on the
	// ChallengeRequest passed as part of the test cases.

	pollTime, _ := time.ParseDuration("15s")
	timeOut, _ := time.ParseDuration("5m")

	fixture := dns.NewFixture(&godaddyDNSSolver{},
		dns.SetResolvedZone(zone),
		dns.SetResolvedFQDN("cert-manager-dns01-test-01.snowdrop.dev."),
		// Increase the poll interval to 15s
		dns.SetPollInterval(pollTime),
		// Increase the limit from 2 min to 5 min as we need more time for the propagation of the TXT Record
		dns.SetPropagationLimit(timeOut),
		dns.SetAllowAmbientCredentials(false),
		dns.SetManifestPath("testdata/godaddy"),
		dns.SetStrict(true),
	)

	fixture.RunConformance(t)
}
