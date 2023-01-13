package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"goose/inceptiondb"
)

func renderHome(staticsDir string) interface{} {

	template, err := t("home", staticsDir, "pages/template.gohtml", "pages/home.gohtml")
	if err != nil {
		panic(err)
	}

	return func(ctx context.Context, w http.ResponseWriter) {

		w.Header().Set(`Link`, `<https://goose.blue/>; rel="canonical"`)

		max := 20
		reader, err := GetInceptionClient(ctx).Find("tweets", inceptiondb.FindQuery{
			Index:   "by timestamp-id",
			Limit:   max,
			Reverse: true,
		})
		if err != nil {
			err = fmt.Errorf("error reading from persistence layer")
		}

		tweets := []JSON{}

		j := json.NewDecoder(reader)
		for {
			tweet := JSON{}
			err := j.Decode(&tweet)
			if err == io.EOF {
				break
			}
			if err != nil {
				err = fmt.Errorf("error decoding %w", err)
			}
			tweets = append(tweets, tweet)
		}

		title := "Goose - La red social libre"
		description := "Goose, la red social realmente libre hasta que el dinero o la ley digan lo contrario, con el código fuente disponible en github.com/dataless-io/goose ¡Aprovéchala!"

		template.Execute(w, map[string]interface{}{
			"title":          title,
			"description":    description,
			"tweets":         tweets,
			"og_title":       title,
			"og_url":         "https://goose.blue/",
			"og_image":       "https://goose.blue/logo-blue-big.png",
			"og_description": description,
		})

	}
}
