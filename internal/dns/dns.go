package dns

import (
	"context"
	"strings"

	"github.com/cert-manager/cert-manager/pkg/issuer/acme/dns/util"
)

func ExtractRecordName(fqdn, domain string) string {
	if idx := strings.Index(fqdn, "."+domain); idx != -1 {
		return fqdn[:idx]
	}
	return util.UnFqdn(fqdn)
}

func GetZone(fqdn string) (string, error) {
	authZone, err := util.FindZoneByFqdn(context.TODO(), fqdn, util.RecursiveNameservers)
	if err != nil {
		return "", err
	}
	return util.UnFqdn(authZone), nil
}
