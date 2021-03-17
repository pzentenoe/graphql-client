package graphql

type graphErr struct {
	Message         string
	ErrorExtensions map[string]interface{} `json:"extensions"`
}

func (e *graphErr) Extensions() map[string]interface{} {
	return e.ErrorExtensions
}
func (e graphErr) Error() string {
	return "graphql: " + e.Message
}
