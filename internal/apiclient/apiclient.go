// Package apiclient implements a client for connecting to BlinkDisk HTTP API server.
package apiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	net_url "net/url"
	"regexp"
	"strings"

	"github.com/pkg/errors"

	"github.com/blinkdisk/core/internal/timetrack"
	"github.com/blinkdisk/core/internal/tlsutil"
	"github.com/blinkdisk/core/repo/logging"
)

var log = logging.Module("client")

// CSRFTokenHeader is the name of CSRF token header that must be sent for most API calls.
//
//nolint:gosec
const CSRFTokenHeader = "X-BlinkDisk-Csrf-Token"

// BlinkDiskAPIClient provides helper methods for communicating with BlinkDisk API server.
type BlinkDiskAPIClient struct {
	BaseURL    string
	HTTPClient *http.Client

	CSRFToken string
}

// Get is a helper that performs HTTP GET on a URL with the specified suffix and decodes the response
// onto respPayload which must be a pointer to byte slice or JSON-serializable structure.
func (c *BlinkDiskAPIClient) Get(ctx context.Context, urlSuffix string, onNotFound error, respPayload any) error {
	return c.runRequest(ctx, http.MethodGet, c.actualURL(urlSuffix), onNotFound, nil, respPayload)
}

// Post is a helper that performs HTTP POST on a URL with the specified body from reqPayload and decodes the response
// onto respPayload which must be a pointer to byte slice or JSON-serializable structure.
func (c *BlinkDiskAPIClient) Post(ctx context.Context, urlSuffix string, reqPayload, respPayload any) error {
	return c.runRequest(ctx, http.MethodPost, c.actualURL(urlSuffix), nil, reqPayload, respPayload)
}

// Put is a helper that performs HTTP PUT on a URL with the specified body from reqPayload and decodes the response
// onto respPayload which must be a pointer to byte slice or JSON-serializable structure.
func (c *BlinkDiskAPIClient) Put(ctx context.Context, urlSuffix string, reqPayload, respPayload any) error {
	return c.runRequest(ctx, http.MethodPut, c.actualURL(urlSuffix), nil, reqPayload, respPayload)
}

// Delete is a helper that performs HTTP DELETE on a URL with the specified body from reqPayload and decodes the response
// onto respPayload which must be a pointer to byte slice or JSON-serializable structure.
func (c *BlinkDiskAPIClient) Delete(ctx context.Context, urlSuffix string, onNotFound error, reqPayload, respPayload any) error {
	return c.runRequest(ctx, http.MethodDelete, c.actualURL(urlSuffix), onNotFound, reqPayload, respPayload)
}

// FetchCSRFTokenForTesting fetches the CSRF token and session cookie for use when making subsequent calls to the API.
// This simulates the browser behavior of downloading the "/" and is required to call the UI-only methods.
func (c *BlinkDiskAPIClient) FetchCSRFTokenForTesting(ctx context.Context) error {
	var b []byte

	if err := c.Get(ctx, "/", nil, &b); err != nil {
		return err
	}

	re := regexp.MustCompile(`<meta name="blinkdisk-csrf-token" content="(.*)" />`)

	match := re.FindSubmatch(b)
	if match == nil {
		return errors.New("CSRF token not found")
	}

	c.CSRFToken = string(match[1])

	return nil
}

func (c *BlinkDiskAPIClient) actualURL(suffix string) string {
	if strings.HasPrefix(suffix, "/") {
		return c.BaseURL + suffix
	}

	return c.BaseURL + "/api/v1/" + suffix
}

func (c *BlinkDiskAPIClient) runRequest(ctx context.Context, method, url string, notFoundError error, reqPayload, respPayload any) error {
	payload, contentType, err := requestReader(reqPayload)
	if err != nil {
		return errors.Wrap(err, "error getting reader")
	}

	req, err := http.NewRequestWithContext(ctx, method, url, payload)
	if err != nil {
		return errors.Wrap(err, "error creating request")
	}

	if c.CSRFToken != "" {
		req.Header.Add(CSRFTokenHeader, c.CSRFToken)
	}

	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "error running http request")
	}

	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode == http.StatusNotFound && notFoundError != nil {
		return notFoundError
	}

	return decodeResponse(resp, respPayload)
}

func requestReader(reqPayload any) (io.Reader, string, error) {
	if reqPayload == nil {
		return nil, "", nil
	}

	if bs, ok := reqPayload.([]byte); ok {
		return bytes.NewReader(bs), "application/octet-stream", nil
	}

	var b bytes.Buffer
	if err := json.NewEncoder(&b).Encode(reqPayload); err != nil {
		return nil, "", errors.Wrap(err, "unable to serialize JSON")
	}

	return bytes.NewReader(b.Bytes()), "application/json", nil
}

// HTTPStatusError encapsulates HTTP status error.
type HTTPStatusError struct {
	HTTPStatusCode int
	ErrorMessage   string
}

func (e HTTPStatusError) Error() string {
	return e.ErrorMessage
}

// serverErrorResponse is a structure that can decode the Error field
// of a serverapi.ErrorResponse received from the API server.
type serverErrorResponse struct {
	Error string `json:"error"`
}

// respToErrorMessage will attempt to JSON decode the response body into
// a structure resembling the serverapi.ErrorResponse struct. If successful,
// the Error field will be included in the string output. Otherwise
// only the response Status field will be returned.
func respToErrorMessage(resp *http.Response) string {
	errResp := serverErrorResponse{}

	err := json.NewDecoder(resp.Body).Decode(&errResp)
	if err != nil {
		return resp.Status
	}

	return fmt.Sprintf("%s: %s", resp.Status, errResp.Error)
}

func decodeResponse(resp *http.Response, respPayload any) error {
	if resp.StatusCode != http.StatusOK {
		return HTTPStatusError{resp.StatusCode, respToErrorMessage(resp)}
	}

	if respPayload == nil {
		return nil
	}

	if b, ok := respPayload.(*[]byte); ok {
		v, err := io.ReadAll(resp.Body)
		if err != nil {
			return errors.Wrap(err, "unable to read response")
		}

		*b = v
	} else if err := json.NewDecoder(resp.Body).Decode(respPayload); err != nil {
		return errors.Wrap(err, "unable to parse JSON response")
	}

	return nil
}

// Options encapsulates all optional parameters for BlinkDiskAPIClient.
type Options struct {
	BaseURL string

	Username string
	Password string

	TrustedServerCertificateFingerprint string

	LogRequests bool
}

// NewBlinkDiskAPIClient creates a client for connecting to BlinkDisk HTTP API.
func NewBlinkDiskAPIClient(options Options) (*BlinkDiskAPIClient, error) {
	var transport http.RoundTripper

	// override transport which trusts only one certificate
	if f := options.TrustedServerCertificateFingerprint; f != "" {
		transport = tlsutil.TransportTrustingSingleCertificate(f)
	} else {
		transport = http.DefaultTransport
	}

	uri := options.BaseURL

	if strings.HasPrefix(options.BaseURL, "unix+https://") || strings.HasPrefix(options.BaseURL, "unix+http://") {
		u, _ := net_url.Parse(strings.TrimPrefix(options.BaseURL, "unix+"))
		uri = u.Scheme + "://localhost"
		tp, _ := transport.(*http.Transport)
		transport = tp.Clone()
		tp, _ = transport.(*http.Transport)
		tp.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
			dial, err := (&net.Dialer{}).DialContext(ctx, "unix", u.Path)
			return dial, errors.Wrap(err, "Failed to connect to socket: "+options.BaseURL)
		}
	}

	// wrap with a round-tripper that provides basic authentication
	if options.Username != "" || options.Password != "" {
		transport = basicAuthTransport{transport, options.Username, options.Password}
	}

	if options.LogRequests {
		transport = loggingTransport{transport}
	}

	cj, err := cookiejar.New(nil)
	if err != nil {
		return nil, errors.Wrap(err, "unable to create cookie jar")
	}

	return &BlinkDiskAPIClient{
		uri,
		&http.Client{
			Jar:       cj,
			Transport: transport,
		},
		"",
	}, nil
}

type basicAuthTransport struct {
	base     http.RoundTripper
	username string
	password string
}

func (t basicAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.SetBasicAuth(t.username, t.password)

	//nolint:wrapcheck
	return t.base.RoundTrip(req)
}

type loggingTransport struct {
	base http.RoundTripper
}

func (t loggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	timer := timetrack.StartTimer()
	resp, err := t.base.RoundTrip(req)
	dur := timer.Elapsed()

	if err != nil {
		log(req.Context()).Debugf("%v %v took %v and failed with %v", req.Method, req.URL, dur, err)
		return nil, errors.Wrap(err, "round-trip error")
	}

	log(req.Context()).Debugf("%v %v took %v and returned %v", req.Method, req.URL, dur, resp.Status)

	return resp, nil
}
