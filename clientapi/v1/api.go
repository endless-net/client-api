package clientapi

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var ErrMapStreamNoEvent = errors.New("map stream returned no event before timeout")

type ControlPlaneStatusError struct {
	Method     string
	Path       string
	StatusCode int
	Status     string
	Body       string
}

func (e *ControlPlaneStatusError) Error() string {
	return fmt.Sprintf("%s %s failed: %s: %s", e.Method, e.Path, e.Status, e.Body)
}

func IsControlPlaneStatus(err error, statusCode int) bool {
	var statusErr *ControlPlaneStatusError
	return errors.As(err, &statusErr) && statusErr.StatusCode == statusCode
}

type API struct {
	BaseURL        string
	BaseURLs       []string
	Token          string
	NodeCredential string
	HTTPClient     *http.Client
}

func NewAPI(baseURL, token string) *API {
	return NewAPIWithBaseURLs([]string{baseURL}, token)
}

func NewAPIWithBaseURLs(baseURLs []string, token string) *API {
	normalized := NormalizeControlPlaneURLs(baseURLs...)
	baseURL := ""
	if len(normalized) > 0 {
		baseURL = normalized[0]
	}
	return &API{
		BaseURL:    baseURL,
		BaseURLs:   normalized,
		Token:      token,
		HTTPClient: NewControlPlaneHTTPClient(15*time.Second, nil),
	}
}

func NewAPIWithNodeCredentialURLs(baseURLs []string, token, nodeCredential string) *API {
	api := NewAPIWithBaseURLs(baseURLs, token)
	api.NodeCredential = strings.TrimSpace(nodeCredential)
	return api
}

func NormalizeControlPlaneURLs(urls ...string) []string {
	seen := map[string]struct{}{}
	normalized := make([]string, 0, len(urls))
	for _, raw := range urls {
		value := strings.TrimRight(strings.TrimSpace(raw), "/")
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	return normalized
}

func NewControlPlaneHTTPClient(timeout time.Duration, rootCAs *x509.CertPool) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = NewControlPlaneTLSConfig(rootCAs)
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func NewControlPlaneTLSConfig(rootCAs *x509.CertPool) *tls.Config {
	return &tls.Config{
		RootCAs:    rootCAs,
		MinVersion: tls.VersionTLS13,
	}
}

func (a *API) BillingPlans() ([]Plan, error) {
	var out struct {
		Plans []Plan `json:"plans"`
	}
	if err := a.request(http.MethodGet, "/billing/plans", nil, &out); err != nil {
		return nil, err
	}
	return out.Plans, nil
}

func (a *API) ListAccounts() ([]Account, error) {
	var out struct {
		Accounts []Account `json:"accounts"`
	}
	if err := a.request(http.MethodGet, "/accounts", nil, &out); err != nil {
		return nil, err
	}
	return out.Accounts, nil
}

func (a *API) AccountEntitlements(accountID string) (EntitlementSet, error) {
	var out EntitlementSet
	if err := a.request(http.MethodGet, "/accounts/"+url.PathEscape(accountID)+"/entitlements", nil, &out); err != nil {
		return EntitlementSet{}, err
	}
	return out, nil
}

func (a *API) AccountUsage(accountID string) (UsageSnapshot, error) {
	var out UsageSnapshot
	if err := a.request(http.MethodGet, "/accounts/"+url.PathEscape(accountID)+"/billing/usage", nil, &out); err != nil {
		return UsageSnapshot{}, err
	}
	return out, nil
}

func (a *API) AccountSubscription(accountID string) (Subscription, error) {
	var out Subscription
	if err := a.request(http.MethodGet, "/accounts/"+url.PathEscape(accountID)+"/billing/subscription", nil, &out); err != nil {
		return Subscription{}, err
	}
	return out, nil
}

func (a *API) CreateCheckout(accountID string, req BillingCheckoutRequest) (CheckoutSession, error) {
	var out CheckoutSession
	if err := a.request(http.MethodPost, "/accounts/"+url.PathEscape(accountID)+"/billing/checkout", req, &out); err != nil {
		return CheckoutSession{}, err
	}
	return out, nil
}

func (a *API) CheckoutSession(accountID, checkoutID string) (CheckoutSession, error) {
	var out CheckoutSession
	if err := a.request(http.MethodGet, "/accounts/"+url.PathEscape(accountID)+"/billing/checkout/"+url.PathEscape(checkoutID), nil, &out); err != nil {
		return CheckoutSession{}, err
	}
	return out, nil
}

func (a *API) ListInvoices(accountID string) ([]Invoice, error) {
	var out struct {
		Invoices []Invoice `json:"invoices"`
	}
	if err := a.request(http.MethodGet, "/accounts/"+url.PathEscape(accountID)+"/billing/invoices", nil, &out); err != nil {
		return nil, err
	}
	return out.Invoices, nil
}

func (a *API) ServerKey() (ServerKeyResponse, error) {
	var out ServerKeyResponse
	return out, a.request(http.MethodGet, "/server-key", nil, &out)
}

func (a *API) CreateNetwork(req CreateNetworkRequest) (Network, error) {
	var out Network
	if strings.TrimSpace(req.IdempotencyKey) == "" {
		key, err := NewCreateIdempotencyKey()
		if err != nil {
			return out, err
		}
		req.IdempotencyKey = key
	}
	return out, a.request(http.MethodPost, "/networks", req, &out)
}

func (a *API) ListNetworks() ([]Network, error) {
	return a.ListNetworksForAccount("")
}

func (a *API) ListNetworksForAccount(accountID string) ([]Network, error) {
	path := "/networks"
	if strings.TrimSpace(accountID) != "" {
		path += "?account_id=" + url.QueryEscape(strings.TrimSpace(accountID))
	}
	var out []Network
	return out, a.request(http.MethodGet, path, nil, &out)
}

func (a *API) ListNodes(network string) ([]Node, error) {
	path := "/networks/" + url.PathEscape(network) + "/nodes"
	var out []Node
	return out, a.request(http.MethodGet, path, nil, &out)
}

func (a *API) ListAdvertisedRoutes(network string) ([]AdvertisedRoute, error) {
	path := "/networks/" + url.PathEscape(network) + "/routes"
	var out []AdvertisedRoute
	return out, a.request(http.MethodGet, path, nil, &out)
}

func (a *API) SetAdvertisedRouteApproval(network string, req SetAdvertisedRouteApprovalRequest) (SetAdvertisedRouteApprovalResponse, error) {
	path := "/networks/" + url.PathEscape(network) + "/routes/approval"
	var out SetAdvertisedRouteApprovalResponse
	return out, a.request(http.MethodPost, path, req, &out)
}

func (a *API) CreateJoinToken(req CreateJoinTokenRequest) (CreateJoinTokenResponse, error) {
	var out CreateJoinTokenResponse
	if strings.TrimSpace(req.IdempotencyKey) == "" {
		key, err := NewCreateIdempotencyKey()
		if err != nil {
			return out, err
		}
		req.IdempotencyKey = key
	}
	return out, a.request(http.MethodPost, "/nodes/join-tokens", req, &out)
}

func (a *API) RegisterNode(req RegisterNodeRequest) (RegisterNodeResponse, error) {
	var out RegisterNodeResponse
	return out, a.request(http.MethodPost, "/nodes/register", req, &out)
}

func (a *API) CreateNodeEnrollmentRequest(req RegisterNodeRequest) (CreateNodeEnrollmentRequestResponse, error) {
	var out CreateNodeEnrollmentRequestResponse
	return out, a.request(http.MethodPost, "/nodes/enrollment-requests", req, &out)
}

func (a *API) NodeEnrollmentRequestStatus(id, pollToken string) (NodeEnrollmentRequestStatusResponse, error) {
	var out NodeEnrollmentRequestStatusResponse
	return out, a.withBearer(pollToken).request(http.MethodGet, "/nodes/enrollment-requests/"+url.PathEscape(strings.TrimSpace(id)), nil, &out)
}

func (a *API) CompleteNodeEnrollmentRequest(id, pollToken string) (CompleteNodeEnrollmentRequestResponse, error) {
	var out CompleteNodeEnrollmentRequestResponse
	return out, a.withBearer(pollToken).request(http.MethodPost, "/nodes/enrollment-requests/"+url.PathEscape(strings.TrimSpace(id))+"/complete", nil, &out)
}

func (a *API) UpdateNodeEndpointState(nodeID string, req UpdateNodeEndpointRequest) (RegisterNodeResponse, error) {
	var out RegisterNodeResponse
	path := "/nodes/" + url.PathEscape(nodeID) + "/endpoint"
	return out, a.request(http.MethodPatch, path, req, &out)
}

func (a *API) ReadMapStreamEvent(nodeID string, fromRevision uint64, timeout time.Duration) (MapStreamEvent, error) {
	capabilities := MapStreamSupportedCapabilities()
	path := fmt.Sprintf(
		"/maps/%s/stream?from_revision=%d&timeout=%s",
		url.PathEscape(nodeID),
		fromRevision,
		url.QueryEscape(timeout.String()),
	)
	var failures []string
	for _, baseURL := range a.controlPlaneURLs() {
		req, err := a.newRequest(http.MethodGet, baseURL, path, nil)
		if err != nil {
			return MapStreamEvent{}, err
		}
		req.Header.Set("X-EndlessNet-Map-Protocol", fmt.Sprintf("%d", MapStreamProtocolVersion))
		req.Header.Set("X-EndlessNet-Map-Capabilities", strings.Join(capabilities, ","))
		resp, err := a.HTTPClient.Do(req)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", baseURL, err))
			continue
		}
		defer func() {
			_ = resp.Body.Close()
		}()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			payload, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			statusErr := &ControlPlaneStatusError{
				Method:     http.MethodGet,
				Path:       path,
				StatusCode: resp.StatusCode,
				Status:     resp.Status,
				Body:       strings.TrimSpace(string(payload)),
			}
			if shouldFailOverStatus(resp.StatusCode) {
				failures = append(failures, fmt.Sprintf("%s: %v", baseURL, statusErr))
				continue
			}
			return MapStreamEvent{}, statusErr
		}
		a.setActiveBaseURL(baseURL)
		if got, want := strings.TrimSpace(resp.Header.Get("X-EndlessNet-Map-Protocol")), strconv.Itoa(MapStreamProtocolVersion); got != want {
			return MapStreamEvent{}, fmt.Errorf("map stream response protocol header = %q, want %q", got, want)
		}
		if got := strings.TrimSpace(resp.Header.Get("X-EndlessNet-Map-Capabilities")); got != strings.Join(capabilities, ",") {
			return MapStreamEvent{}, fmt.Errorf("map stream response capabilities header = %q, want %q", got, strings.Join(capabilities, ","))
		}
		reader := bufio.NewReader(resp.Body)
		for {
			line, err := readBoundedLine(reader, MaxControlMapStreamEventBytes, "control map stream event")
			if err != nil {
				if isEOFWithNoData(line, err) {
					return MapStreamEvent{}, ErrMapStreamNoEvent
				}
				return MapStreamEvent{}, err
			}
			var event MapStreamEvent
			if err := decodeStrictMapStreamEvent(line, &event); err != nil {
				return MapStreamEvent{}, err
			}
			if event.ProtocolVersion != MapStreamProtocolVersion {
				return MapStreamEvent{}, fmt.Errorf("map stream protocol = %d, want %d", event.ProtocolVersion, MapStreamProtocolVersion)
			}
			if !mapStreamCapabilitiesEqual(event.Capabilities, capabilities) {
				return MapStreamEvent{}, fmt.Errorf("map stream event capabilities = %v, want %v", event.Capabilities, capabilities)
			}
			switch event.Type {
			case "heartbeat":
				continue
			case "snapshot", "resync":
				return event, nil
			default:
				return MapStreamEvent{}, fmt.Errorf("unsupported map stream event type %q", event.Type)
			}
		}
	}
	return MapStreamEvent{}, failoverError(http.MethodGet, path, failures)
}

func decodeStrictMapStreamEvent(raw []byte, event *MapStreamEvent) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(event); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("map stream event contains trailing JSON")
		}
		return err
	}
	return nil
}

func mapStreamCapabilitiesEqual(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for index := range got {
		if got[index] != want[index] {
			return false
		}
	}
	return true
}

func (a *API) DeleteNode(id string) error {
	return a.request(http.MethodDelete, "/nodes/"+url.PathEscape(id), nil, nil)
}

func (a *API) Logout() error {
	return a.request(http.MethodPost, "/auth/logout", nil, nil)
}

func (a *API) request(method, path string, in any, out any) error {
	var raw []byte
	if in != nil {
		var err error
		raw, err = json.Marshal(in)
		if err != nil {
			return err
		}
	}
	var failures []string
	for _, baseURL := range a.controlPlaneURLs() {
		req, err := a.newRequest(method, baseURL, path, raw)
		if err != nil {
			return err
		}
		if in != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := a.HTTPClient.Do(req)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", baseURL, err))
			continue
		}
		defer func() {
			_ = resp.Body.Close()
		}()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			payload, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			statusErr := &ControlPlaneStatusError{
				Method:     method,
				Path:       path,
				StatusCode: resp.StatusCode,
				Status:     resp.Status,
				Body:       strings.TrimSpace(string(payload)),
			}
			if shouldFailOverStatus(resp.StatusCode) {
				failures = append(failures, fmt.Sprintf("%s: %v", baseURL, statusErr))
				continue
			}
			return statusErr
		}
		a.setActiveBaseURL(baseURL)
		if out == nil || resp.StatusCode == http.StatusNoContent {
			return nil
		}
		return decodeStrictAPIResponse(resp.Body, out)
	}
	return failoverError(method, path, failures)
}

func decodeStrictAPIResponse(reader io.Reader, out any) error {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(out); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("control-plane response contains trailing JSON")
		}
		return err
	}
	return nil
}

func (a *API) newRequest(method, baseURL, path string, raw []byte) (*http.Request, error) {
	if err := validateControlPlaneBaseURL(baseURL); err != nil {
		return nil, err
	}
	var body io.Reader
	if raw != nil {
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, baseURL+path, body)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(a.Token) != "" {
		req.Header.Set("Authorization", "Bearer "+a.Token)
	}
	if strings.TrimSpace(a.NodeCredential) != "" {
		req.Header.Set("X-EndlessNet-Node-Credential", strings.TrimSpace(a.NodeCredential))
	}
	return req, nil
}

func validateControlPlaneBaseURL(rawURL string) error {
	rawURL = strings.TrimSpace(rawURL)
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" || parsed.Hostname() == "" {
		return fmt.Errorf("invalid control-plane URL %q", rawURL)
	}
	if parsed.User != nil {
		return errors.New("control-plane URL must not contain user information")
	}
	switch strings.ToLower(parsed.Scheme) {
	case "https":
		return nil
	case "http":
		host := strings.TrimSuffix(strings.ToLower(parsed.Hostname()), ".")
		ip := net.ParseIP(host)
		if host == "localhost" || (ip != nil && ip.IsLoopback()) {
			return nil
		}
		return errors.New("control-plane URL must use HTTPS outside loopback development")
	default:
		return errors.New("control-plane URL must use HTTP or HTTPS")
	}
}

func (a *API) controlPlaneURLs() []string {
	urls := NormalizeControlPlaneURLs(append([]string{a.BaseURL}, a.BaseURLs...)...)
	if len(urls) > 0 {
		return urls
	}
	return nil
}

func (a *API) setActiveBaseURL(baseURL string) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return
	}
	a.BaseURL = baseURL
	a.BaseURLs = NormalizeControlPlaneURLs(append([]string{baseURL}, a.BaseURLs...)...)
}

func (a *API) withBearer(token string) *API {
	clone := *a
	clone.Token = strings.TrimSpace(token)
	clone.NodeCredential = ""
	return &clone
}

func shouldFailOverStatus(statusCode int) bool {
	return statusCode == http.StatusRequestTimeout || statusCode == http.StatusTooManyRequests || statusCode >= 500
}

func failoverError(method, path string, failures []string) error {
	if len(failures) == 0 {
		return fmt.Errorf("%s %s failed: control plane URL is required", method, path)
	}
	return fmt.Errorf("%s %s failed against all coordinators: %s", method, path, strings.Join(failures, "; "))
}
