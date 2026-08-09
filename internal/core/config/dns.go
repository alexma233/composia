package config

import (
	"errors"
	"fmt"
	"strings"
)

type dnsZoneConfig struct {
	provider string
	zone     string
}

func (controller *ControllerConfig) ResolveDNSProvider(hostname string) (string, error) {
	if controller == nil || controller.DNS == nil {
		return "", errors.New("controller dns is not configured")
	}
	fqdn := normalizeDNSName(hostname)
	bestZone := ""
	provider := ""
	for _, candidate := range controller.dnsZones() {
		if !dnsNameMatchesZone(fqdn, candidate.zone) {
			continue
		}
		if len(candidate.zone) < len(bestZone) {
			continue
		}
		if len(candidate.zone) == len(bestZone) && provider != "" && provider != candidate.provider {
			return "", fmt.Errorf("hostname %q matches equally specific DNS zone %q in providers %q and %q", hostname, candidate.zone, provider, candidate.provider)
		}
		bestZone = candidate.zone
		provider = candidate.provider
	}
	if provider == "" {
		return "", fmt.Errorf("no configured DNS zone matched hostname %q", hostname)
	}
	return provider, nil
}

func dnsZones(controller *ControllerDNSConfig) []dnsZoneConfig {
	if controller == nil {
		return nil
	}
	var zones []dnsZoneConfig
	appendZones := func(provider string, rawZones []string) {
		for _, rawZone := range rawZones {
			if zone := normalizeDNSName(rawZone); zone != "" {
				zones = append(zones, dnsZoneConfig{provider: provider, zone: zone})
			}
		}
	}
	if controller.Cloudflare != nil {
		appendZones("cloudflare", controller.Cloudflare.Zones)
	}
	if controller.AliDNS != nil {
		appendZones("alidns", controller.AliDNS.Zones)
	}
	if controller.DNSPod != nil {
		appendZones("dnspod", controller.DNSPod.Zones)
	}
	if controller.Route53 != nil {
		appendZones("route53", controller.Route53.Zones)
	}
	if controller.HuaweiCloud != nil {
		appendZones("huaweicloud", controller.HuaweiCloud.Zones)
	}
	return zones
}

func validateControllerDNS(controller *ControllerDNSConfig) error {
	if controller == nil {
		return nil
	}
	seen := make(map[string]string)
	for _, candidate := range dnsZones(controller) {
		if previous, ok := seen[candidate.zone]; ok && previous != candidate.provider {
			return fmt.Errorf("controller.dns zone %q is configured for both %s and %s", candidate.zone, previous, candidate.provider)
		}
		seen[candidate.zone] = candidate.provider
	}
	return nil
}

func (controller *ControllerConfig) dnsZones() []dnsZoneConfig {
	if controller == nil {
		return nil
	}
	return dnsZones(controller.DNS)
}

func normalizeDNSName(value string) string {
	return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(value), "."))
}

func dnsNameMatchesZone(name, zone string) bool {
	return name == zone || strings.HasSuffix(name, "."+zone)
}
