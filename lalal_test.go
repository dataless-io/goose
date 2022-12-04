package goose

import (
	"html/template"
	"os"
	"testing"

	"goose/statics"
)

func p(name string, codes ...string) (t *template.Template, err error) {

	t = template.New(name)

	for _, code := range codes {
		t, err = t.Parse(code)
		// TODO: return on error
	}

	return
}

func q(name, staticsDir string, filenames ...string) (t *template.Template, err error) {

	f := statics.FileReader(staticsDir)

	t = template.New(name)

	for _, filename := range filenames {

		data, err := f(filename)
		if err != nil {
			return nil, err
		}

		t, err = t.Parse(string(data))
		if err != nil {
			return nil, err
		}
	}

	return
}

func TestLalalal(t *testing.T) {

	t_home, err := q("home", "./statics/www/", "pages/template.gohtml", "pages/home.gohtml")

	if err != nil {
		panic(err)
	}

	t_home.Execute(os.Stdout, map[string]interface{}{
		"title": "Home page",
		"name":  "Fulanezxxx",
	})

}

// t_home, err := p("home",
// 	`
// 		This is the template
// 		{{block "content" .}}
// 			THIS IS THE CONTENT
// 		{{end}}
// 	`,
// 	`
// 		{{define "content"}}
// 			Hello this is t_home
// 			name: {{ .name }}
// 		{{end}}
// 	`,
// )
