package unionewrapper_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/unione-repo/unione-go-sdk/api/webhook"
	"github.com/unione-repo/unione-go-sdk/client"
	unionewrapper "github.com/unione-repo/unione-nethttp"
)

const testKey = "test-key"

// sample is a webhook notification with two events; Sign replaces its auth placeholder.
const sample = `{
	"auth": "00000000000000000000000000000000",
	"events_by_user": [
		{
			"user_id": 1,
			"project_id": "6432890213745872",
			"events": [
				{
					"event_name": "transactional_email_status",
					"event_data": {
						"job_id": "1a3Q2V-0000OZ-S0",
						"email": "user@example.com",
						"status": "delivered",
						"event_time": "2015-11-30 15:09:42"
					}
				},
				{
					"event_name": "transactional_spam_block",
					"event_data": {
						"block_time": "2015-11-30 15:09:42",
						"block_type": "one_smtp",
						"domain": "example.com",
						"SMTP_blocks_count": 1,
						"domain_status": "blocked"
					}
				}
			]
		}
	]
}`

func TestMain(m *testing.M) {
	log.SetOutput(io.Discard) // WriteError logs the whole error
	os.Exit(m.Run())
}

// request is what the API mock received.
type request struct {
	path, apiKey, platform string
}

// apiMock is an API on 127.0.0.1 answering every request with status and body.
func apiMock(t *testing.T, status int, body string) (*httptest.Server, *request) {
	t.Helper()
	got := &request{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.path, got.platform = r.URL.Path, r.URL.Query().Get("platform")
		got.apiKey = r.Header.Get("X-API-KEY")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(server.Close)
	return server, got
}

// setEnv sets every UNIONE_* variable of the package (empty ones are unset for ConfigFromEnv).
func setEnv(t *testing.T, apiKey, host, timeout, webhookKey, webhookPath string) {
	t.Setenv(unionewrapper.EnvAPIKey, apiKey)
	t.Setenv(unionewrapper.EnvHost, host)
	t.Setenv(unionewrapper.EnvTimeout, timeout)
	t.Setenv(unionewrapper.EnvWebhookKey, webhookKey)
	t.Setenv(unionewrapper.EnvWebhookPath, webhookPath)
}

func TestConfigFromEnv(t *testing.T) {
	setEnv(t, testKey, "http://127.0.0.1:1/api", "2.5", "webhook-key", "/hooks/mail")
	c := unionewrapper.ConfigFromEnv()
	if c.APIKey != testKey || c.Host != "http://127.0.0.1:1/api" || c.Timeout != 2500*time.Millisecond {
		t.Errorf("config %+v", c)
	}
	if c.SigningKey() != "webhook-key" || c.Path() != "/hooks/mail" {
		t.Errorf("signing key %q, path %q", c.SigningKey(), c.Path())
	}
}

func TestConfigDefaults(t *testing.T) {
	setEnv(t, testKey, "", "not a number", "", "")
	c := unionewrapper.ConfigFromEnv()
	if c.Timeout != 0 || c.SigningKey() != testKey || c.Path() != unionewrapper.DefaultWebhookPath {
		t.Errorf("timeout %v, signing key %q, path %q", c.Timeout, c.SigningKey(), c.Path())
	}
}

func TestNewClientFromEnvWithoutKey(t *testing.T) {
	setEnv(t, "", "", "", "", "")
	if c, err := unionewrapper.NewClientFromEnv(); c != nil || !errors.Is(err, unionewrapper.ErrNoAPIKey) {
		t.Errorf("client %v, error %v", c, err)
	}
}

func TestClientFromEnvInContext(t *testing.T) {
	server, got := apiMock(t, 200, `{"status":"success"}`)
	setEnv(t, testKey, server.URL+"/api/v1", "", "", "")
	c, err := unionewrapper.NewClientFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	var callErr error
	handler := unionewrapper.ClientMiddleware(c)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, callErr = unionewrapper.ClientFromContext(r.Context()).System.Ping(r.Context())
	}))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	if callErr != nil {
		t.Fatal(callErr)
	}
	if got.path != "/api/v1/system/ping.json" || got.apiKey != testKey {
		t.Errorf("request %+v", got)
	}
	if got.platform != unionewrapper.Platform+","+client.Platform {
		t.Errorf("platform %q", got.platform)
	}
	if unionewrapper.ClientFromContext(context.Background()) != nil {
		t.Error("a client in a context without the middleware")
	}
}

// newWebhook returns the handler with the key and counters of the notifications and events it got.
func newWebhook(key string, fail bool) (http.Handler, *int, *[]string) {
	notifications, emails := 0, []string{}
	handler := unionewrapper.WebhookHandler(key, unionewrapper.Handlers{
		OnNotification: func(context.Context, *webhook.WebhookCallback) error {
			notifications++
			return nil
		},
		OnEvent: func(_ context.Context, e *webhook.WebhookCallbackEventsByUserItemEventsItem,
			_ *webhook.WebhookCallbackEventsByUserItem) error {
			if fail {
				return errors.New("queue is down")
			}
			email := ""
			if e.EventData != nil {
				email = e.EventData.Email
			}
			emails = append(emails, email)
			return nil
		},
	})
	return handler, &notifications, &emails
}

func serve(handler http.Handler, method string, body []byte) int {
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(method, unionewrapper.DefaultWebhookPath, bytes.NewReader(body)))
	return rec.Code
}

func TestWebhookURLCheck(t *testing.T) {
	handler, notifications, _ := newWebhook(testKey, false)
	for method, want := range map[string]int{http.MethodGet: 200, http.MethodHead: 200, http.MethodPut: 405} {
		if code := serve(handler, method, nil); code != want {
			t.Errorf("%s: %d, want %d", method, code, want)
		}
	}
	if *notifications != 0 {
		t.Error("a notification without POST")
	}
}

func TestWebhookSigned(t *testing.T) {
	handler, notifications, emails := newWebhook(testKey, false)
	if code := serve(handler, http.MethodPost, unionewrapper.Sign([]byte(sample), testKey)); code != 200 {
		t.Fatalf("POST: %d", code)
	}
	if *notifications != 1 || len(*emails) != 2 || (*emails)[0] != "user@example.com" {
		t.Errorf("%d notifications, events %v", *notifications, *emails)
	}
}

func TestWebhookWrongSignature(t *testing.T) {
	handler, notifications, _ := newWebhook(testKey, false)
	signed := unionewrapper.Sign([]byte(sample), testKey)
	bodies := map[string][]byte{
		"tampered":  bytes.Replace(signed, []byte("user@"), []byte("evil@"), 1),
		"other key": unionewrapper.Sign([]byte(sample), "other-key"),
		"unsigned":  []byte(sample),
	}
	for name, body := range bodies {
		if code := serve(handler, http.MethodPost, body); code != 401 {
			t.Errorf("%s: %d, want 401", name, code)
		}
	}
	noKey, _, _ := newWebhook("", false)
	if code := serve(noKey, http.MethodPost, signed); code != 401 {
		t.Errorf("empty key: %d, want 401", code)
	}
	if *notifications != 0 {
		t.Error("a notification with a wrong signature reached the handlers")
	}
}

func TestWebhookHandlerError(t *testing.T) {
	handler, _, _ := newWebhook(testKey, true)
	if code := serve(handler, http.MethodPost, unionewrapper.Sign([]byte(sample), testKey)); code != 500 {
		t.Errorf("POST: %d, want 500 (the API repeats the notification)", code)
	}
}

func TestWriteError(t *testing.T) {
	const secret = "Unknown recipient of the account"
	server, _ := apiMock(t, 400, `{"status":"error","message":"`+secret+`","code":204}`)
	host, ctx := server.URL+"/api/v1", context.Background()
	_, apiErr := unionewrapper.Config{APIKey: testKey, Host: host}.Client().System.Ping(ctx)
	_, invalid := unionewrapper.Config{Host: host}.Client().System.Ping(ctx) // no key
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"api", apiErr, http.StatusBadGateway},
		{"invalid request", invalid, http.StatusInternalServerError},
		{"timeout", &client.APIError{Message: "system/ping: HTTP request timed out", Kind: client.KindTimeout},
			http.StatusGatewayTimeout},
		{"network", &client.APIError{Message: "system/ping: HTTP request failed", Kind: client.KindNetwork},
			http.StatusBadGateway},
		{"other", errors.New("boom"), http.StatusBadGateway},
	}
	for _, tc := range cases {
		if tc.err == nil {
			t.Fatalf("%s: no error", tc.name)
		}
		rec := httptest.NewRecorder()
		unionewrapper.WriteError(rec, tc.err)
		if rec.Code != tc.want || strings.TrimSpace(rec.Body.String()) != "email sending failed" {
			t.Errorf("%s: %d %q, want %d", tc.name, rec.Code, rec.Body.String(), tc.want)
		}
	}
}
