# UniOne Go net/http 1.1.3

Web API 1.90

Quickstart guide: [Send emails with Go](https://docs.unione.io/en/send-with-go) · Docs for LLMs: [llms.txt](https://docs.unione.io/llms.txt) · [API reference](https://docs.unione.io/en/web-api-ref)

Go net/http integration of the [UniOne Go SDK](https://github.com/unione-repo/unione-go-sdk) (`github.com/unione-repo/unione-go-sdk`): the API client in the request context (net/http middleware), a webhook endpoint with signature check.

## Install

```bash
go get github.com/unione-repo/unione-nethttp
```

Requirements: Go 1.21+.

## Get an API key

1. Sign up at [cp.unione.io](https://cp.unione.io/en/user/registration/).
2. Copy your API key from [Settings → Security → API](https://cp.unione.io/en/settings/security/api).
3. Put it in an environment variable:

```bash
export UNIONE_API_KEY="your-api-key"
```

The package reads it from `UNIONE_API_KEY` (see the configuration below).

> **Server-side only.** Use this package in backend code: API routes, server functions, workers.
> Never ship the API key to a browser or a mobile app, and never commit it.

## Configuration

Settings are read from the environment by default:

```bash
UNIONE_API_KEY=YOUR_API_KEY
UNIONE_HOST=us1.unione.io
```

| Option | Environment variable | Default | Description |
|---|---|---|---|
| `APIKey` | `UNIONE_API_KEY` |  | User or project API key (required). |
| `Host` | `UNIONE_HOST` | `api.unione.io` | API host (datacenter: `api.unione.io`, `eu1.unione.io`, `us1.unione.io`) or a full base URL. |
| `Timeout` | `UNIONE_TIMEOUT` | `30` | Request timeout, seconds. |
| `WebhookKey` | `UNIONE_WEBHOOK_KEY` |  | API key the webhooks are signed with (the key of the user/project of the webhook), if it differs from the API key. |
| `WebhookPath` | `UNIONE_WEBHOOK_PATH` | `/unione/webhook` | Path of the webhook endpoint. |

`ConfigFromEnv()` reads the variables; a `Config` can also be filled in code. `UNIONE_TIMEOUT` is in seconds, `Config.Timeout` is a `time.Duration`.

## Send your first email — no DNS setup

Every account can send test email right away from a **sandbox domain**: pre-verified, DKIM and SPF already set.
It delivers only to addresses you confirm.

The examples use the API client of the package (see the configuration above).

**1. Get your sandbox domain** (`domain/get-sandbox-domain`). The call creates the domain on first use and returns the same one afterwards.

```go
import (
	"fmt"
	"net/http"

	unionewrapper "github.com/unione-repo/unione-nethttp"
)

mux.HandleFunc("/sandbox-domain", func(w http.ResponseWriter, r *http.Request) {
	client := unionewrapper.ClientFromContext(r.Context())
	response, err := client.Domain.GetSandboxDomain(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	fmt.Fprintln(w, response.Domain)
})
```

**2. Confirm your inbox** (`confirmed-email/create`). UniOne sends a confirmation email; click the link in it within 48 hours.

```go
import (
	"fmt"
	"net/http"

	"github.com/unione-repo/unione-go-sdk/api/confirmedemail"
	unionewrapper "github.com/unione-repo/unione-nethttp"
)

mux.HandleFunc("/confirm-email", func(w http.ResponseWriter, r *http.Request) {
	client := unionewrapper.ClientFromContext(r.Context())
	response, err := client.ConfirmedEmail.Create(r.Context(), &confirmedemail.ConfirmedEmailRequest{
		Email: "you@example.com",
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	fmt.Fprintln(w, response.Status)
})
```

Check the status with `confirmed-email/list` — your address should show `"status": "confirmed"`.

**3. Send the email** (`email/send`) from any address on the sandbox domain to the confirmed address.

```go
import (
	"fmt"
	"net/http"

	unione "github.com/unione-repo/unione-go-sdk"
	"github.com/unione-repo/unione-go-sdk/api/email"
	"github.com/unione-repo/unione-go-sdk/common"
	unionewrapper "github.com/unione-repo/unione-nethttp"
)

mux.HandleFunc("/send", func(w http.ResponseWriter, r *http.Request) {
	client := unionewrapper.ClientFromContext(r.Context())
	response, err := client.Email.Send(r.Context(), &email.EmailSendRequest{
		Message: &email.MessageObject{
			Recipients: []email.MessageObjectRecipientsItem{
				{
					Email: "you@example.com",
				},
			},
			Body: &common.BodyObject{
				HTML: unione.String("<p>It works!</p>"),
			},
			Subject:   "Hello from UniOne",
			FromEmail: unione.String("noreply@<sandbox-domain>"),
		},
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	fmt.Fprintln(w, response.JobID)
})
```

### What you get back

```json
{
  "status": "success",
  "job_id": "1ZymBc-00041N-9X",
  "emails": [
    "you@example.com"
  ]
}
```

On error the API returns `status: "error"`, a `message` and a numeric `code`:

```json
{
  "status": "error",
  "code": 102,
  "message": "Error ID:83274F98-C7D5-11F1-B221-6214FF8707B1. Can not decode key wrongkey"
}
```

### Same flow with curl

```bash
BASE=https://api.unione.io/en/transactional/api/v1

curl -s -X POST $BASE/domain/get-sandbox-domain.json \
  -H "Content-Type: application/json" -H "X-API-KEY: $UNIONE_API_KEY" -d '{}'

curl -s -X POST $BASE/confirmed-email/create.json \
  -H "Content-Type: application/json" -H "X-API-KEY: $UNIONE_API_KEY" \
  -d '{"email": "you@example.com"}'

curl -s -X POST $BASE/email/send.json \
  -H "Content-Type: application/json" -H "X-API-KEY: $UNIONE_API_KEY" \
  -d '{
        "message": {
          "recipients": [
            {
              "email": "you@example.com"
            }
          ],
          "from_email": "noreply@<sandbox-domain>",
          "subject": "Hello from UniOne",
          "body": {
            "html": "<p>It works!</p>"
          }
        }
      }'
```

## Webhooks

The endpoint answers `GET` with 200 OK (the URL check of [webhook/set](https://docs.unione.io/en/web-api-ref#webhook-set)) and accepts the `POST` notifications: it checks the `auth` field (MD5 of the request body in which the `auth` value is replaced by the API key), answers 401 to a wrong signature, and passes the notification and each of its events to the application.

```go
import (
	"context"
	"log"

	"github.com/unione-repo/unione-go-sdk/api/webhook"
	unionewrapper "github.com/unione-repo/unione-nethttp"
)

cfg := unionewrapper.ConfigFromEnv()
handler := unionewrapper.WebhookHandler(cfg.SigningKey(), unionewrapper.Handlers{
	OnEvent: func(ctx context.Context, e *webhook.WebhookCallbackEventsByUserItemEventsItem, user *webhook.WebhookCallbackEventsByUserItem) error {
		if e.EventData != nil {
			log.Printf("UniOne event %s: %s %s", e.EventName, e.EventData.Email, e.EventData.Status)
		}
		return nil
	},
})

mux.Handle(cfg.Path(), handler)           // net/http, chi: r.Handle(cfg.Path(), handler)
router.Any(cfg.Path(), gin.WrapH(handler)) // gin
e.Any(cfg.Path(), echo.WrapHandler(handler)) // echo
```

Register the URL of the endpoint with `webhook/set`, e.g. `https://example.com/unione/webhook`.

## API client

Every section of the API is available on the client, see the [SDK docs](https://github.com/unione-repo/unione-go-sdk).

```go
import (
	"fmt"
	"net/http"

	unionewrapper "github.com/unione-repo/unione-nethttp"
)

cfg := unionewrapper.ConfigFromEnv()
client := cfg.Client()

mux := http.NewServeMux()
mux.HandleFunc("/usage", func(w http.ResponseWriter, r *http.Request) {
	info, err := unionewrapper.ClientFromContext(r.Context()).System.Info(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	fmt.Fprintln(w, info.Email)
})
http.ListenAndServe(":8080", unionewrapper.ClientMiddleware(client)(mux))
```

## Sending through the API

The client calls [email/send](https://docs.unione.io/en/web-api-ref#email-send) with all of its options: templates, substitutions, metadata, tracking. Fields of the request and other examples: [Go SDK docs](https://github.com/unione-repo/unione-go-sdk/blob/HEAD/docs/Email.md#send).

```go
import (
	"fmt"
	"net/http"

	unione "github.com/unione-repo/unione-go-sdk"
	"github.com/unione-repo/unione-go-sdk/api/email"
	"github.com/unione-repo/unione-go-sdk/common"
	unionewrapper "github.com/unione-repo/unione-nethttp"
)

mux.HandleFunc("/send", func(w http.ResponseWriter, r *http.Request) {
	client := unionewrapper.ClientFromContext(r.Context())
	response, err := client.Email.Send(r.Context(), &email.EmailSendRequest{
		Message: &email.MessageObject{
			Recipients: []email.MessageObjectRecipientsItem{
				{
					Email: "user@example.com",
					Substitutions: map[string]string{
						"CustomerId": "12452",
						"to_name":    "John Smith",
					},
				},
			},
			Body: &common.BodyObject{
				HTML:      unione.String("<b>Hello, {{to_name}}</b>"),
				Plaintext: unione.String("Hello, {{to_name}}"),
			},
			Subject:   "UniOne test email ",
			FromEmail: unione.String("email@example.com"),
			FromName:  unione.String("John Smith"),
		},
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	fmt.Fprintln(w, response.JobID)
})
```

## Sandbox limits

- Sends only to confirmed addresses: up to 10 per account.
- Up to 100 emails a day.
- The sandbox domain expires after 31 days; call `domain/get-sandbox-domain` again to get a new one.
- Works only with the account API key, not a project key.
- Not listed in `domain/list`; the other `domain/*` methods do not apply to it.

The sandbox is for development and testing. Do not use a sandbox domain as the sender in production.

## Before production: verify your domain

1. `domain/get-dns-records` — returns the values for your DNS records.
2. Add these records at your DNS provider (the domain page of the control panel shows them too):

   | Type | Host | Value |
   |---|---|---|
   | TXT | `@` | the `verification-record` value (`unione-validate-hash=…`) |
   | TXT | `<selector>._domainkey` (the selector is on the domain page of the control panel, e.g. `us`) | `v=DKIM1; k=rsa; p=<dkim>` |
   | TXT | `@` | `v=spf1 include:spf.unione.io ~all` |
   | CNAME | `_dmarc` | the `dmarc` value |

3. `domain/validate-verification-record`, then `domain/validate-dkim`.
4. `domain/list` — wait until the verification record is confirmed and DKIM is active.
5. Read `from_email` from your config, so switching from the sandbox needs no code change.

## Sending reliably

- **Retries:** set `message.idempotence_key` on sends that may be retried, so a repeated request does not send twice.
- **Limits:** up to 500 recipients and 10 MB per request; each attachment up to 7 MB.
- **Body:** always include `body.html` or `body.plaintext`.
- **Delivery status:** register a webhook (`webhook/set`) with a fast, idempotent HTTPS handler instead of polling.
- **Defaults:** keep the default unsubscribe footer and tracking; changing them requires UniOne support.
- **Errors:** retry `429` and `5xx` with backoff; do not retry other errors unchanged.

## Links

- [Send emails with Go](https://docs.unione.io/en/send-with-go) and other [integration guides](https://docs.unione.io/en/integrations/)
- [Web API reference](https://docs.unione.io/en/web-api-ref)
- [llms.txt](https://docs.unione.io/llms.txt) for AI coding agents
- [UniOne MCP server and agent setup](https://unione.io/en/agents)
- [Support](https://cp.unione.io/en/support)

## For package contributors

See [AGENTS.md](AGENTS.md): how this package is generated from the OpenAPI spec, built, tested and released.
