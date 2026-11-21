package proxy

import (
	"fmt"
	"os"
)

// BrightDataCDPURL returns a CDP websocket URL for Bright Data's Browser API,
// or "" if not configured. Env: BRIGHTDATA_CDP_URL (full URL) or
// BRIGHTDATA_CUSTOMER + BRIGHTDATA_ZONE + BRIGHTDATA_PASSWORD.
func BrightDataCDPURL() string {
	if u := os.Getenv("BRIGHTDATA_CDP_URL"); u != "" {
		return u
	}
	customer := os.Getenv("BRIGHTDATA_CUSTOMER")
	zone := os.Getenv("BRIGHTDATA_ZONE")
	password := os.Getenv("BRIGHTDATA_PASSWORD")
	if customer == "" || zone == "" || password == "" {
		return ""
	}
	return fmt.Sprintf("wss://brd-customer-%s-zone-%s:%s@brd.superproxy.io:9222", customer, zone, password)
}