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

// API calls the control-plane HTTP contract. Token is sent as a bearer token;
// NodeCredential is sent separately as X-EndlessNet-Node-Credential. Producers
// authorize each operation; configuring both does not make them interchangeable.
// Requests fail over across configured origins on transport errors, HTTP 408,
// HTTP 429 and 5xx, including for writes. Persist mutation IDs before calling.
// JSON responses reject unknown fields and trailing JSON. Non-retried HTTP
// failures return ControlPlaneStatusError; exhausted failover returns an
// aggregate error. Recovery decisions require decoding and validating PublicError,
// not matching diagnostic text or relying on HTTP status alone.
// Successful calls update the active origin; API is not safe for concurrent use.
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

// ServerKey reads GET /server-key and returns the server key projection. Fetching
// a key over HTTP does not establish signing trust; credential and map verification
// must use the caller's trusted signing bundles.
func (a *API) ServerKey() (ServerKeyResponse, error) {
	var out ServerKeyResponse
	return out, a.request(http.MethodGet, "/server-key", nil, &out)
}

// CreateNetwork posts the requested name, address ranges, DNS and optional account
// and cell selection to /networks, returning the created network. The producer
// checks create rights and resource constraints. If IdempotencyKey is empty, the
// SDK generates one for this invocation only. Persist and supply an explicit key
// with unchanged input when retrying across calls or process restarts.
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

// ListNetworks reads GET /networks without an account filter, returning the
// networks exposed to the caller by the producer. It is equivalent to
// ListNetworksForAccount(""); this HTTP response is an array, not an RPC page.
func (a *API) ListNetworks() ([]Network, error) {
	return a.ListNetworksForAccount("")
}

// ListNetworksForAccount reads GET /networks with an optional account_id query
// filter. Whitespace is trimmed and a blank ID omits the filter. The producer
// still enforces account/network access. The result is an unpaginated HTTP array.
func (a *API) ListNetworksForAccount(accountID string) ([]Network, error) {
	path := "/networks"
	if strings.TrimSpace(accountID) != "" {
		path += "?account_id=" + url.QueryEscape(strings.TrimSpace(accountID))
	}
	var out []Network
	return out, a.request(http.MethodGet, path, nil, &out)
}

// ListNodes reads GET /networks/{network}/nodes for an authorized network ID and
// returns node metadata as an array. Listing nodes does not grant connectivity;
// peer configuration and traffic authorization come from the verified signed map.
func (a *API) ListNodes(network string) ([]Node, error) {
	path := "/networks/" + url.PathEscape(network) + "/nodes"
	var out []Node
	return out, a.request(http.MethodGet, path, nil, &out)
}

// CreateJoinToken posts network selection, TTL, enrollment flags and tags to
// /nodes/join-tokens. The producer authorizes issuance; the result contains the
// secret token, effective scope/options and expiry. Keep the token confidential.
// An empty IdempotencyKey is generated for this call only; supply a persisted key
// and unchanged input to retry token creation across separate calls safely.
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

// RegisterNode posts direct enrollment or credential renewal to /nodes/register.
// Enrollment binds the device proof to join-token or session authorization;
// renewal uses the old node credential and saved registration binding. Persist
// IdempotencyID and the signed request before sending, and reuse them on retries.
// The SDK validates the request before sending and the response's identity and
// operation binding on success. The caller must additionally verify credential
// and map signatures with trusted bundles before committing the returned state.
// See RECOVERY.md for typed recovery errors and durable retry requirements.
func (a *API) RegisterNode(req RegisterNodeRequest) (RegisterNodeResponse, error) {
	var out RegisterNodeResponse
	if err := req.Validate(); err != nil {
		return out, err
	}
	if err := a.request(http.MethodPost, "/nodes/register", req, &out); err != nil {
		return out, err
	}
	return out, out.ValidateForRequest(req)
}

// CreateNodeEnrollmentRequest posts a browser-mediated enrollment request to
// /nodes/enrollment-requests. Persist the registration operation ID before sending.
// The result contains request state, an approval URL, a secret PollToken and the
// suggested polling delay. Browser approval/poll authorization is separate from
// direct enrollment; this method does not invoke RegisterNodeRequest.Validate.
// Creation alone does not enroll the node; poll and complete the approved request.
func (a *API) CreateNodeEnrollmentRequest(req RegisterNodeRequest) (CreateNodeEnrollmentRequestResponse, error) {
	var out CreateNodeEnrollmentRequestResponse
	return out, a.request(http.MethodPost, "/nodes/enrollment-requests", req, &out)
}

// NodeEnrollmentRequestStatus reads GET /nodes/enrollment-requests/{id} using
// only pollToken as bearer authorization, clearing the configured node credential
// on a copy of the API. It returns request state and the suggested next polling
// delay. Pending, approved, rejected, expired and enrolled are defined states;
// approval is not yet a registration result. The original API credentials persist.
func (a *API) NodeEnrollmentRequestStatus(id, pollToken string) (NodeEnrollmentRequestStatusResponse, error) {
	var out NodeEnrollmentRequestStatusResponse
	return out, a.withBearer(pollToken).request(http.MethodGet, "/nodes/enrollment-requests/"+url.PathEscape(strings.TrimSpace(id)), nil, &out)
}

// CompleteNodeEnrollmentRequest posts to /nodes/enrollment-requests/{id}/complete
// using only the request's poll token, without changing this API's credentials.
// It requests completion of approved browser enrollment and returns request state
// plus an optional registration. Check for a registration before using it and
// validate the saved operation/device binding and trusted signatures before
// persisting credentials or applying the map; this method only decodes the body.
func (a *API) CompleteNodeEnrollmentRequest(id, pollToken string) (CompleteNodeEnrollmentRequestResponse, error) {
	var out CompleteNodeEnrollmentRequestResponse
	return out, a.withBearer(pollToken).request(http.MethodPost, "/nodes/enrollment-requests/"+url.PathEscape(strings.TrimSpace(id))+"/complete", nil, &out)
}

// UpdateNodeEndpointState patches /nodes/{nodeID}/endpoint with the node's
// endpoint, generation, candidates, TTL and optional status/client version.
// The producer must bind the operation to the authenticated node and validate
// endpoint state. The response uses RegisterNodeResponse as its map projection;
// this method only strictly decodes JSON and does not call ValidateForRequest.
// Authenticate the returned map before applying it to local network state.
func (a *API) UpdateNodeEndpointState(nodeID string, req UpdateNodeEndpointRequest) (RegisterNodeResponse, error) {
	var out RegisterNodeResponse
	path := "/nodes/" + url.PathEscape(nodeID) + "/endpoint"
	return out, a.request(http.MethodPatch, path, req, &out)
}

// ReadMapStreamEvent reads /maps/{nodeID}/stream from the supplied network/global
// revision and map hash using the configured node authorization. It negotiates
// the current map protocol/capabilities, skips heartbeats, and returns the first
// snapshot, delta, checkpoint or resync event, closing the stream afterwards.
// timeout is sent to the server; HTTPClient.Timeout independently bounds the call.
// EOF before an event returns ErrMapStreamNoEvent. Framing and event shape are
// checked, but signature verification and delta reconstruction remain the caller's
// responsibility. Commit a cursor only with verified state; request a full snapshot
// when the base cannot be reconstructed or authenticated.
func (a *API) ReadMapStreamEvent(nodeID string, cursor MapCursor, timeout time.Duration) (MapStreamEvent, error) {
	capabilities := MapStreamSupportedCapabilities()
	path := fmt.Sprintf(
		"/maps/%s/stream?from_network_revision=%d&from_global_revision=%d&from_map_hash=%s&timeout=%s",
		url.PathEscape(nodeID),
		cursor.Revision.Network,
		cursor.Revision.Global,
		url.QueryEscape(strings.TrimSpace(cursor.MapHash)),
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
			if err := validateMapStreamEvent(event); err != nil {
				return MapStreamEvent{}, err
			}
			switch event.Type {
			case "heartbeat":
				continue
			case "snapshot", "delta", "checkpoint", "resync":
				return event, nil
			default:
				return MapStreamEvent{}, fmt.Errorf("unsupported map stream event type %q", event.Type)
			}
		}
	}
	return MapStreamEvent{}, failoverError(http.MethodGet, path, failures)
}

func validateMapStreamEvent(event MapStreamEvent) error {
	if strings.TrimSpace(event.EventID) == "" {
		return errors.New("map stream event_id is required")
	}
	switch event.Type {
	case "heartbeat":
		if event.Snapshot != nil || event.Delta != nil {
			return errors.New("map stream heartbeat must not include a snapshot or delta")
		}
	case "snapshot", "resync":
		if !event.HasFullMap() {
			return fmt.Errorf("map stream %s event is missing a complete snapshot", event.Type)
		}
		if event.Delta != nil {
			return fmt.Errorf("map stream %s event must not include a delta", event.Type)
		}
		if event.ResultSignature == nil {
			return fmt.Errorf("map stream %s event is missing result_signature", event.Type)
		}
	case "delta":
		if event.Delta == nil {
			return errors.New("map stream delta event is missing delta")
		}
		if event.Snapshot != nil {
			return errors.New("map stream delta event must not include a snapshot")
		}
		if strings.TrimSpace(event.BaseHash) == "" {
			return errors.New("map stream delta event is missing base_hash")
		}
		if event.ResultSignature == nil {
			return errors.New("map stream delta event is missing result_signature")
		}
	case "checkpoint":
		if event.Snapshot != nil || event.Delta != nil {
			return errors.New("map stream checkpoint must not include a snapshot or delta")
		}
	default:
		return fmt.Errorf("unsupported map stream event type %q", event.Type)
	}
	return nil
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

// Logout posts /auth/logout to invalidate the authenticated session at the
// producer. A successful response has no decoded payload. This call does not
// clear Token, NodeCredential or persisted local state; the caller manages local
// sign-out. A failed request does not confirm server-side session revocation.
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
