package graphql

import (
	"fmt"
	"io"
	"net/http"
)

// GraphRequest is a GraphQL request.
type GraphRequest struct {
	query  string
	vars   map[string]interface{}
	files  []File
	Header http.Header
}

// NewGraphqlRequest makes a new GraphRequest with the specified query string.
func NewGraphqlRequest(query string) *GraphRequest {
	req := &GraphRequest{
		query:  query,
		Header: make(map[string][]string),
	}
	return req
}

// Var sets a variable.
func (req *GraphRequest) Var(key string, value interface{}) {
	if req.vars == nil {
		req.vars = make(map[string]interface{})
	}
	req.vars[key] = value
}

// Vars gets the variables for this GraphRequest.
func (req *GraphRequest) Vars() map[string]interface{} {
	return req.vars
}

// Files gets the files in this request.
func (req *GraphRequest) Files() []File {
	return req.files
}

// Query gets the query string of this request.
func (req *GraphRequest) Query() string {
	return req.query
}

// File sets a file to upload.
// Files are only supported with a Client that was created with
// the UseMultipartForm option.
func (req *GraphRequest) File(fieldname, filename string, r io.Reader) {
	req.files = append(req.files, File{
		Field: fieldname,
		Name:  filename,
		R:     r,
	})
}

// File represents a file to upload.
type File struct {
	Field string
	Name  string
	R     io.Reader
}

func (c *Client) logf(format string, args ...interface{}) {
	c.Log(fmt.Sprintf(format, args...))
}
