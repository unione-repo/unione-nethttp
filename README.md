# UniOne Go net/http

[![Go net/http](https://pkg.go.dev/badge/github.com/unione-repo/unione-nethttp.svg)](https://pkg.go.dev/github.com/unione-repo/unione-nethttp) [![License: MIT](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

Go net/http integration of the [UniOne Go SDK](https://github.com/unione-repo/unione-go-sdk) (`github.com/unione-repo/unione-go-sdk`): the API client in the request context (net/http middleware), a webhook endpoint with signature check.

[API reference](https://docs.unione.io/en/web-api-ref) · Docs for LLMs: [llms.txt](https://docs.unione.io/llms.txt)

## Install

```bash
go get github.com/unione-repo/unione-nethttp
```

Requirements: Go 1.21+.

## Configuration

Get an API key in the [control panel](https://cp.unione.io/en/settings/security/api) and keep it in the environment, never in code:

```bash
export UNIONE_API_KEY="your-api-key"
```

Settings are read from the environment by default:

```bash
UNIONE_API_KEY=YOUR_API_KEY
UNIONE_HOST=us1.unione.io
```

`us1.unione.io` in the example is for an account on another datacenter; without the host setting the package uses `api.unione.io`.

| Option | Environment variable | Default | Description |
|---|---|---|---|
| `APIKey` | `UNIONE_API_KEY` |  | User or project API key (required). |
| `Host` | `UNIONE_HOST` | `api.unione.io` | API host (datacenter: `api.unione.io`, `eu1.unione.io`, `us1.unione.io`) or a full base URL. |
| `Timeout` | `UNIONE_TIMEOUT` | `30` | Request timeout, seconds. |
| `WebhookKey` | `UNIONE_WEBHOOK_KEY` |  | API key the webhooks are signed with (the key of the user/project of the webhook), if it differs from the API key. |
| `WebhookPath` | `UNIONE_WEBHOOK_PATH` | `/unione/webhook` | Path of the webhook endpoint. |

`ConfigFromEnv()` reads the variables; a `Config` can also be filled in code. `UNIONE_TIMEOUT` is in seconds, `Config.Timeout` is a `time.Duration`.

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

> **Answer 200 within 3 seconds** and process the events in the background (a queue): otherwise UniOne repeats the request every 10 minutes for 24 hours (`retry_count` grows), and a handler that failed every request for at least 10 events of a day is turned inactive. The handler must take at least **5 parallel requests of up to 100 events each**; deduplicate, a repeat delivers the same events again.

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

### Sending through the API

The client calls [email/send](https://docs.unione.io/en/web-api-ref#email-send) with all of its options: templates, substitutions, metadata, tracking. Fields of the request and other examples: [Go SDK docs](https://github.com/unione-repo/unione-go-sdk/blob/HEAD/docs/Email.md#send).

```go
import (
	"net/http"

	unione "github.com/unione-repo/unione-go-sdk"
	"github.com/unione-repo/unione-go-sdk/api/email"
	"github.com/unione-repo/unione-go-sdk/common"
	unionewrapper "github.com/unione-repo/unione-nethttp"
)

mux.HandleFunc("/send", func(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	client := unionewrapper.ClientFromContext(ctx)

	if _, err := client.Email.Send(ctx, &email.EmailSendRequest{
		Message: &email.MessageObject{
			Recipients: []email.MessageObjectRecipientsItem{email.RecipientFromAddress("Alex <alex@example.com>")},
			FromEmail:  unione.String("shop@example.com"),
			FromName:   unione.String("Shop"),
			Subject:    "Your order",
			Body:       common.HTMLBody("<p>Hello, {{to_name}}! Your order has shipped.</p>"),
		},
	}); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
})
```

## No domain yet: the test sandbox domain

Until your domain is verified, send from the **sandbox domain**: a test domain every account has, already verified, with DKIM and SPF set. It delivers only to addresses you confirm (the confirmation link comes by email, valid 48 hours), up to 100 emails a day. The sandbox domain is deleted 31 days after it was created, but you can get it again: the next `domain/get-sandbox-domain` call creates a new one (with a new name, so read it from the call, do not hardcode it).

The examples use the API client of the package (see the configuration above).

```go
import (
	"net/http"

	unione "github.com/unione-repo/unione-go-sdk"
	"github.com/unione-repo/unione-go-sdk/api/confirmedemail"
	"github.com/unione-repo/unione-go-sdk/api/email"
	"github.com/unione-repo/unione-go-sdk/common"
	unionewrapper "github.com/unione-repo/unione-nethttp"
)

mux.HandleFunc("/sandbox", func(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	client := unionewrapper.ClientFromContext(ctx)

	// 1. Your sandbox domain: created on the first call, the same one afterwards
	sandbox, err := client.Domain.GetSandboxDomain(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	// 2. Confirm your inbox: UniOne sends a link, click it within 48 hours
	confirm := &confirmedemail.ConfirmedEmailRequest{Email: "you@example.com"}
	if _, err := client.ConfirmedEmail.Create(ctx, confirm); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	// 3. Send from any address on the sandbox domain to the confirmed one
	if _, err := client.Email.Send(ctx, &email.EmailSendRequest{
		Message: &email.MessageObject{
			Recipients: []email.MessageObjectRecipientsItem{email.RecipientFromAddress("you@example.com")},
			FromEmail:  unione.String("noreply@" + sandbox.Domain),
			Subject:    "Hello from UniOne",
			Body:       common.HTMLBody("<p>It works!</p>"),
		},
	}); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
})
```

<details>
<summary>Sandbox limits</summary>

- Sends only to confirmed addresses: up to 10 per account.
- Up to 100 emails a day.
- The sandbox domain expires after 31 days; call `domain/get-sandbox-domain` again to get a new one.
- Works only with the account API key, not a project key.
- Not listed in `domain/list`; the other `domain/*` methods do not apply to it.

The sandbox is for development and testing. Do not use a sandbox domain as the sender in production.

</details>

<details>
<summary>The same with curl</summary>

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

</details>

## Going to production

**1. Verify your sending domain.** `domain/get-dns-records` returns the verification record, the DKIM key and the DMARC target; add the records at your DNS provider, then `domain/validate-verification-record` and `domain/validate-dkim`.

| Type | Host | Value | Where it comes from |
|---|---|---|---|
| TXT | `@` | `unione-validate-hash=…` | `verification-record` of the response, as is |
| TXT | `<selector>._domainkey` (the selector is on the domain page of the control panel, e.g. `us`) | `v=DKIM1; k=rsa; p=<dkim>` | `dkim` of the response after `v=DKIM1; k=rsa; p=` |
| TXT | `@` | `v=spf1 include:spf.unione.io ~all` | not in the response; add the include to your SPF record if you have one |
| CNAME | `_dmarc` | `<domain>.dmarc.…` | `dmarc` of the response (UniOne publishes the DMARC policy); keep your own DMARC TXT record instead if the domain has one |

**2. Make sending reliable.**

| | |
|---|---|
| Retries | `429` and `5xx` with backoff and the same `idempotence_key`; fix other errors instead of retrying |
| Limits | 500 recipients and 10 MB per request, attachments included: base64 makes a file about a third larger |
| Body | always `html` or `plaintext` (a template can supply it) |
| Delivery status | a webhook, not polling |
| Queue | send from a background job, not inside the web request |
| Config | `from_email` from config, so the sandbox → production switch needs no code change |
| Defaults | keep the unsubscribe footer and tracking; changing them needs UniOne support |

## Help

- [Integration guides](https://docs.unione.io/en/integrations)
- [Go SDK](https://github.com/unione-repo/unione-go-sdk)
- AI coding agents: [llms.txt](https://docs.unione.io/llms.txt), the [MCP server](https://unione.io/en/agents), skills: `npx skills add https://github.com/unione-repo/unione-skills.git`
- [Support](https://cp.unione.io/en/support) · [Issues](https://github.com/unione-repo/unione-nethttp/issues)
- Contributing: [AGENTS.md](AGENTS.md) — the package is generated from the OpenAPI spec.
