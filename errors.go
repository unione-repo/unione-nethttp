package unionewrapper

import (
	"errors"
	"log"
	"net/http"

	"github.com/unione-repo/unione-go-sdk/client"
)

// WriteError answers a request whose API call failed, by the kind of the error of the client (client.APIError.Kind):
// 500 for client.KindInvalidRequest (a bug of the request), 504 Gateway Timeout for client.KindTimeout, 502 Bad
// Gateway for the others, with the generic body "email sending failed"; the whole error (it may have the API
// response) only goes to the log.
//
//	if err != nil {
//		unionewrapper.WriteError(w, err)
//		return
//	}
func WriteError(w http.ResponseWriter, err error) {
	status := http.StatusBadGateway
	var apiErr *client.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.Kind {
		case client.KindInvalidRequest:
			status = http.StatusInternalServerError
		case client.KindTimeout:
			status = http.StatusGatewayTimeout
		}
	}
	log.Printf("%s: %v", Platform, err)
	http.Error(w, "email sending failed", status)
}
