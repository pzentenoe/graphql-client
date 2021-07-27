package graphql

type GraphErr struct {
	Message         string
	ErrorExtensions map[string]interface{} `json:"extensions"`
	Locations       []Location             `json:"locations"`
	Path            []string               `json:"path"`
}
type Location struct {
	Column int `json:"column"`
	Line   int `json:"line"`
}

func (e *GraphErr) Extensions() map[string]interface{} {
	return e.ErrorExtensions
}
func (e GraphErr) Error() string {
	return "graphql: " + e.Message
}
