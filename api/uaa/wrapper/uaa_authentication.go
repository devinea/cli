package wrapper

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"strings"

	"code.cloudfoundry.org/cli/api/uaa"
)

//go:generate counterfeiter . UAAClient

type UAAClient interface {
	AccessToken() string
	RefreshToken() error
}

// UAAAuthentication wraps connections and adds authentication headers to all
// requests
type UAAAuthentication struct {
	connection uaa.Connection
	client     UAAClient
}

// NewUAAAuthentication returns a pointer to a UAAAuthentication wrapper with
// the client set as the AuthenticationStore
func NewUAAAuthentication(client UAAClient) *UAAAuthentication {
	return &UAAAuthentication{
		client: client,
	}
}

// Wrap sets the connection on the UAAAuthentication and returns itself
func (t *UAAAuthentication) Wrap(innerconnection uaa.Connection) uaa.Connection {
	t.connection = innerconnection
	return t
}

// Make adds authentication headers to the passed in request and then calls the
// wrapped connection's Make
func (t *UAAAuthentication) Make(request *http.Request, passedResponse *uaa.Response) error {
	var err error
	var rawRequestBody []byte

	if request.Body != nil {
		rawRequestBody, err = ioutil.ReadAll(request.Body)
		defer request.Body.Close()
		if err != nil {
			return err
		}

		request.Body = ioutil.NopCloser(bytes.NewBuffer(rawRequestBody))

		// The authentication header is not added to the token refresh request.
		if strings.Contains(request.URL.String(), "/oauth/token") &&
			request.Method == http.MethodPost &&
			strings.Contains(string(rawRequestBody), "grant_type=refresh_token") {
			return t.connection.Make(request, passedResponse)
		}
	}

	request.Header.Set("Authorization", t.client.AccessToken())

	err = t.connection.Make(request, passedResponse)
	if _, ok := err.(uaa.InvalidAuthTokenError); ok {
		err = t.client.RefreshToken()
		if err != nil {
			return err
		}

		if rawRequestBody != nil {
			request.Body = ioutil.NopCloser(bytes.NewBuffer(rawRequestBody))
		}
		request.Header.Set("Authorization", t.client.AccessToken())
		return t.connection.Make(request, passedResponse)
	}

	return err
}
