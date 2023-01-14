package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/fulldump/box"

	"goose/glueauth"
	"goose/inceptiondb"
)

func renderHonk(staticsDir string) interface{} {

	template, err := t("honk", staticsDir, "pages/template.gohtml", "pages/honk.gohtml")
	if err != nil {
		panic(err)
	}

	return func(ctx context.Context, w http.ResponseWriter) {

		userHandle := box.GetUrlParameter(ctx, "user-handle")
		honkId := box.GetUrlParameter(ctx, "honk-id")

		selfUrl := `https://goose.blue/user/` + url.PathEscape(userHandle) + `/honk/` + url.PathEscape(honkId)
		w.Header().Set(`Link`, `<`+selfUrl+`>; rel="canonical"`)

		var honk Tweet
		findErr := GetInceptionClient(ctx).FindOne("honks", inceptiondb.FindQuery{
			Index: "by id",
			Value: honkId,
		}, &honk)
		if findErr != nil {
			// todo: render a pretty and user friendly error page
			w.WriteHeader(http.StatusNotFound)
			return
		}

		if honk.Nick != userHandle {
			// todo: render a pretty and user friendly error page
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// fetch followers
		followers := map[string]bool{}
		auth := glueauth.GetAuth(ctx)
		if auth != nil {

			followerID := auth.User.ID
			// followerID := "user-123" // TODO: remove this

			// TODO: optimize using the user from the current honk
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

		words := strings.SplitN(honk.Message, " ", 6)

		title := "@" + userHandle + ": " + strings.Join(words[0:len(words)-1], " ") + "..."

		template.Execute(w, map[string]interface{}{
			"title":          title,
			"description":    honk.Message,
			"name":           userHandle,
			"honk":           honk,
			"followers":      followers,
			"og_title":       title,
			"og_url":         selfUrl,
			"og_image":       honk.Picture,
			"og_description": honk.Message,
		})
	}
}
