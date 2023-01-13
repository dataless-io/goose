package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"goose/glueauth"
	"goose/inceptiondb"
)

func renderHome(staticsDir string) interface{} {

	template, err := t("home", staticsDir, "pages/template.gohtml", "pages/home.gohtml")
	if err != nil {
		panic(err)
	}

	return func(ctx context.Context, w http.ResponseWriter) {

		w.Header().Set(`Link`, `<https://goose.blue/>; rel="canonical"`)

		// fetch tweets
		tweets := []JSON{}
		max := 50
		reader, err := GetInceptionClient(ctx).Find("tweets", inceptiondb.FindQuery{
			Index:   "by timestamp-id",
			Limit:   max,
			Reverse: true,
		})
		if err != nil {
			err = fmt.Errorf("error reading from persistence layer")
		}
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

		// fetch followers
		followers := map[string]bool{}
		auth := glueauth.GetAuth(ctx)
		if auth != nil || true {

			// followerID := auth.User.ID
			followerID := "user-123"

			reader, err := GetInceptionClient(ctx).Find("followers", inceptiondb.FindQuery{
				Index: "by follower",
				Limit: 100, // TODO: this max number of followers...
				From: JSON{
					"follower_id": followerID,
					"user_id":     "",
				},
				To: JSON{
					"follower_id": followerID,
					"user_id":     "z",
				},
			})
			if err != nil {
				log.Println("ERROR: fetch followers:", err.Error())
			}
			defer reader.Close()

			j := json.NewDecoder(reader)
			for {
				relationship := struct {
					UserID     string `json:"user_id"`
					FollowerID string `json:"follower_id"`
				}{}
				err := j.Decode(&relationship)
				if err == io.EOF {
					break
				}
				if err != nil {
					err = fmt.Errorf("error decoding %w", err)
				}
				followers[relationship.UserID] = true
			}
		}

		title := "Goose - La red social libre"
		description := "Goose, la red social realmente libre hasta que el dinero o la ley digan lo contrario, con el código fuente disponible en github.com/dataless-io/goose ¡Aprovéchala!"

		err = template.Execute(w, map[string]interface{}{
			"title":          title,
			"description":    description,
			"tweets":         tweets,
			"followers":      followers,
			"og_title":       title,
			"og_url":         "https://goose.blue/",
			"og_image":       "https://goose.blue/logo-blue-big.png",
			"og_description": description,
		})
		if err != nil {
			log.Println(err.Error())
		}
	}
}
