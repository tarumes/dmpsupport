package rive

import (
	"bytes"
	"log"
	"strings"
	"text/template"
)

type RiveScript struct {
	Begin  Begin
	Topics map[string]Topic
}
type Begin struct {
	Global   map[string]string
	Var      map[string]string
	Sub      map[string]string
	Person   map[string]string
	Array    map[string][]string
	Triggers []Trigger
}
type Trigger struct {
	Trigger   string
	Reply     []string
	Condition []string
	Redirect  string
	Previous  string
}
type Topic struct {
	Inherits []string
	Includes []string
	Objects  map[string]string
	Triggers []Trigger
}

var brainz *template.Template = template.Must(template.New("").Funcs(
	template.FuncMap{
		"stringsJoin": func(in []string, sep string) string {
			return strings.Join(in, sep)
		},
		"stringsReplaceAll": func(in string, old string, new string) string {
			return strings.ReplaceAll(in, old, new)
		},
	},
).Parse(`
> begin
 + request
- {ok}
<begin

// Bot Variables
{{ range $key, $value := .Begin.Var }}! var {{$key}} = {{$value}}
{{ end }}
// Substitutions
{{ range $key, $value := .Begin.Sub }}! sub {{$key}} = {{$value}}
{{ end }}
// Person Substitutions
{{ range $key, $value := .Begin.Person }}! person {{$key}} = {{$value}}
{{ end }}
// Arrays
{{ range $key, $value := .Begin.Array }}! array {{$key}} = {{ stringsReplaceAll (stringsJoin $value "|") " " "\\s"  }}
{{ end }}
// Topics
{{ range $key,$val := .Topics }}> topic {{$key}}{{if $val.Includes}} includes{{range $val.Includes}} {{.}}{{end}}{{end}}{{if $val.Inherits}} inherits{{range $val.Inherits}} {{.}}{{end}}{{end}}
{{range $val.Triggers}}
  + {{.Trigger}}
{{if .Previous}} % {{.Previous}}
{{end}}{{if .Redirect}}  @ {{.Redirect}}
{{end}}{{if .Condition}}{{range .Condition}}  * {{.}}
{{end}}{{end}}{{range .Reply}}  - {{.}}
{{end}}{{end}}
< topic{{ end }}
`))

func MakeBrain(brain RiveScript) string {
	b := new(bytes.Buffer)
	err := brainz.Execute(b, brain)
	if err != nil {
		log.Fatal(err)
	}
	return b.String()
}

func (c *Client) LoadBrain() RiveScript {

	return RiveScript{}
}
