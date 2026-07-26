package wgkeys

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ValidateSafeText rejects characters which can split configuration lines or
// otherwise alter the interpretation of a generated WireGuard configuration.
func ValidateSafeText(field, value string) error {
	if !utf8.ValidString(value) {
		return fmt.Errorf("%s contains invalid UTF-8", field)
	}
	for _, r := range value {
		if unicode.IsControl(r) || r == '\u2028' || r == '\u2029' {
			return fmt.Errorf("%s contains a control character", field)
		}
	}
	return nil
}

// ValidatePublicKey requires the canonical WireGuard key encoding: padded
// standard Base64 which decodes to exactly 32 bytes.
func ValidatePublicKey(value string) error {
	return validateKey("wireguard public key", value)
}

func ValidatePrivateKey(value string) error {
	return validateKey("wireguard private key", value)
}

func validateKey(field, value string) error {
	if err := ValidateSafeText(field, value); err != nil {
		return err
	}
	if value != strings.TrimSpace(value) || value == "" {
		return fmt.Errorf("%s must not be empty or contain surrounding whitespace", field)
	}
	raw, err := base64.StdEncoding.Strict().DecodeString(value)
	if err != nil || len(raw) != 32 || base64.StdEncoding.EncodeToString(raw) != value {
		return fmt.Errorf("%s must be canonical standard base64 encoding of 32 bytes", field)
	}
	return nil
}

// ValidateHostname accepts an RFC 1123 host name without a trailing root dot.
func ValidateHostname(value string) error {
	if err := ValidateSafeText("hostname", value); err != nil {
		return err
	}
	if value == "" || value != strings.TrimSpace(value) {
		return errors.New("hostname must not be empty or contain surrounding whitespace")
	}
	if len(value) > 253 {
		return errors.New("hostname must be at most 253 characters")
	}
	for _, label := range strings.Split(value, ".") {
		if len(label) == 0 || len(label) > 63 {
			return errors.New("hostname labels must contain between 1 and 63 characters")
		}
		for i := 0; i < len(label); i++ {
			c := label[i]
			if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || (c == '-' && i > 0 && i < len(label)-1) {
				continue
			}
			return errors.New("hostname must use RFC 1123 letters, digits, dots, and non-edge hyphens")
		}
	}
	return nil
}

// ValidateEndpoint requires host:port. IPv6 literals must use the bracketed
// form accepted by net.SplitHostPort; address zones are deliberately rejected.
func ValidateEndpoint(value string) error {
	if err := ValidateSafeText("endpoint", value); err != nil {
		return err
	}
	if value == "" || value != strings.TrimSpace(value) {
		return errors.New("endpoint must not be empty or contain surrounding whitespace")
	}
	host, portText, err := net.SplitHostPort(value)
	if err != nil {
		return fmt.Errorf("endpoint must be host:port with bracketed IPv6: %w", err)
	}
	if host == "" || strings.Contains(host, "%") {
		return errors.New("endpoint host is invalid")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 || strconv.Itoa(port) != portText {
		return errors.New("endpoint port must be a decimal number between 1 and 65535")
	}
	if addr, err := netip.ParseAddr(host); err == nil {
		if !addr.IsValid() {
			return errors.New("endpoint IP address is invalid")
		}
		return nil
	}
	if err := ValidateHostname(host); err != nil {
		return fmt.Errorf("endpoint host: %w", err)
	}
	return nil
}

func ValidateDNSIP(value string) error {
	if err := ValidateSafeText("DNS address", value); err != nil {
		return err
	}
	if value == "" || value != strings.TrimSpace(value) {
		return errors.New("DNS address must not be empty or contain surrounding whitespace")
	}
	addr, err := netip.ParseAddr(value)
	if err != nil || !addr.IsValid() || addr.Zone() != "" {
		return errors.New("DNS address must be an IPv4 or IPv6 address")
	}
	return nil
}

func ValidateAddr(field, value string) error {
	if err := ValidateSafeText(field, value); err != nil {
		return err
	}
	if value == "" || value != strings.TrimSpace(value) {
		return fmt.Errorf("%s must not be empty or contain surrounding whitespace", field)
	}
	addr, err := netip.ParseAddr(value)
	if err != nil || !addr.IsValid() || addr.Zone() != "" {
		return fmt.Errorf("%s must be an IPv4 or IPv6 address", field)
	}
	return nil
}

func ValidatePrefix(field, value string) error {
	if err := ValidateSafeText(field, value); err != nil {
		return err
	}
	if value == "" || value != strings.TrimSpace(value) {
		return fmt.Errorf("%s must not be empty or contain surrounding whitespace", field)
	}
	prefix, err := netip.ParsePrefix(value)
	if err != nil || !prefix.IsValid() || prefix.Addr().Zone() != "" {
		return fmt.Errorf("%s must be an IPv4 or IPv6 prefix", field)
	}
	return nil
}
