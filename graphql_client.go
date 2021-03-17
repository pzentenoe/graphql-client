package graphql

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"

	"github.com/pkg/errors"
)

type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Client is a client for interacting with a GraphQL API.
type Client struct {
	// graphql url
	url            string
	query          string
	params         map[string]interface{}
	headers        http.Header
	httpClient     HTTPDoer
	graphqlRequest *GraphqlRequest

	useMultipartForm bool

	// closeReq will close the request body immediately allowing for reuse of client
	closeReq bool

	// Log is called with various debug information.
	// To log to standard out, use:
	//  client.Log = func(s string) { log.Println(s) }
	Log func(s string)
}

// NewClient makes a new Client capable of making GraphQL requests.
func NewClient(url string, opts ...ClientOption) *Client {
	c := &Client{
		url: url,
		Log: func(string) {},
	}
	for _, optionFunc := range opts {
		optionFunc(c)
	}
	if c.httpClient == nil {
		c.httpClient = http.DefaultClient
	}

	return c
}

func (c *Client) Run(ctx context.Context, req *GraphqlRequest, resp, errorResp interface{}) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if len(req.files) > 0 && !c.useMultipartForm {
		return errors.New("cannot send files with PostFields option")
	}
	if c.useMultipartForm {
		return c.runWithPostFields(ctx, req, resp, errorResp)
	}
	return c.runWithJSON(ctx, req, resp, errorResp)
}

type graphqlModel struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables"`
}

// WithHTTPClient specifies the underlying http.Client to use when
// making requests.
//  NewClient(url, WithHTTPClient(specificHTTPClient))
func WithHTTPClient(httpclient HTTPDoer) ClientOption {
	return func(client *Client) {
		client.httpClient = httpclient
	}
}

// UseMultipartForm uses multipart/form-data and activates support for
// files.
func UseMultipartForm() ClientOption {
	return func(client *Client) {
		client.useMultipartForm = true
	}
}

// ImmediatelyCloseReqBody will close the req body immediately after each request body is ready
func ImmediatelyCloseReqBody() ClientOption {
	return func(client *Client) {
		client.closeReq = true
	}
}

// ClientOption are functions that are passed into NewClient to
// modify the behaviour of the Client.
type ClientOption func(*Client)

type graphResponse struct {
	Data   interface{}
	Errors []graphErr
}

func (c *Client) runWithJSON(ctx context.Context, req *GraphqlRequest, resp, errorResp interface{}) error {
	var requestBody bytes.Buffer
	requestBodyObj := graphqlModel{
		Query:     req.query,
		Variables: req.vars,
	}
	if err := json.NewEncoder(&requestBody).Encode(requestBodyObj); err != nil {
		return errors.Wrap(err, "encode body")
	}
	c.logf(">> variables: %v", req.vars)
	c.logf(">> query: %s", req.query)
	gr := &graphResponse{Data: resp}

	r, err := http.NewRequest(http.MethodPost, c.url, &requestBody)
	if err != nil {
		return err
	}

	r.Close = c.closeReq
	addHTTPHeaders(r, req, "application/json; charset=utf-8")
	c.logf(">> headers: %v", r.Header)
	r = r.WithContext(ctx)
	res, err := c.httpClient.Do(r)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, res.Body); err != nil {
		return errors.Wrap(err, "reading body")
	}
	c.logf("<< %s", buf.String())
	if res.StatusCode != http.StatusOK {
		_ = json.Unmarshal(buf.Bytes(), &errorResp)
		return fmt.Errorf("graphql: server returned a non-200 status code: %v", res.StatusCode)
	}
	if err := json.NewDecoder(&buf).Decode(&gr); err != nil {
		return errors.Wrap(err, "decoding response")
	}
	if len(gr.Errors) > 0 {
		return gr.Errors[0]
	}
	return nil
}

func (c *Client) runWithPostFields(ctx context.Context, req *GraphqlRequest, resp, errorResp interface{}) error {
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)
	if err := writer.WriteField("query", req.query); err != nil {
		return errors.Wrap(err, "write query field")
	}
	var variablesBuf bytes.Buffer
	if len(req.vars) > 0 {
		variablesField, err := writer.CreateFormField("variables")
		if err != nil {
			return errors.Wrap(err, "create variables field")
		}
		if err := json.NewEncoder(io.MultiWriter(variablesField, &variablesBuf)).Encode(req.vars); err != nil {
			return errors.Wrap(err, "encode variables")
		}
	}
	for i := range req.files {
		part, err := writer.CreateFormFile(req.files[i].Field, req.files[i].Name)
		if err != nil {
			return errors.Wrap(err, "create form file")
		}
		if _, err := io.Copy(part, req.files[i].R); err != nil {
			return errors.Wrap(err, "preparing file")
		}
	}
	if err := writer.Close(); err != nil {
		return errors.Wrap(err, "close writer")
	}
	c.logf(">> variables: %s", variablesBuf.String())
	c.logf(">> files: %d", len(req.files))
	c.logf(">> query: %s", req.query)
	gr := &graphResponse{
		Data: resp,
	}
	r, err := http.NewRequest(http.MethodPost, c.url, &requestBody)
	if err != nil {
		return err
	}
	r.Close = c.closeReq
	addHTTPHeaders(r, req, writer.FormDataContentType())
	c.logf(">> headers: %v", r.Header)
	r = r.WithContext(ctx)
	res, err := c.httpClient.Do(r)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, res.Body); err != nil {
		return errors.Wrap(err, "reading body")
	}
	c.logf("<< %s", buf.String())
	if err := json.NewDecoder(&buf).Decode(&gr); err != nil {
		if res.StatusCode != http.StatusOK {
			_ = json.Unmarshal(buf.Bytes(), &errorResp)
			return fmt.Errorf("graphql: server returned a non-200 status code: %v", res.StatusCode)
		}
		return errors.Wrap(err, "decoding response")
	}
	if len(gr.Errors) > 0 {
		// return first error
		return gr.Errors[0]
	}
	return nil
}

func addHTTPHeaders(httpRequest *http.Request, req *GraphqlRequest, contentType string) {
	httpRequest.Header.Set("Content-Type", contentType)
	httpRequest.Header.Set("Accept", "application/json; charset=utf-8")
	for key, values := range req.Header {
		for _, value := range values {
			httpRequest.Header.Add(key, value)
		}
	}
}
