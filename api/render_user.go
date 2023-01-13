package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/fulldump/box"

	"goose/inceptiondb"
)

func renderUser(staticsDir string) interface{} {

	template, err := t("home", staticsDir, "pages/template.gohtml", "pages/user.gohtml")
	if err != nil {
		panic(err)
	}

	return func(ctx context.Context, w http.ResponseWriter) {

		userHandle := box.GetUrlParameter(ctx, "user-handle")

		selfUrl := `https://goose.blue/user/` + url.PathEscape(userHandle)
		w.Header().Set(`Link`, `<`+selfUrl+`>; rel="canonical"`)

		user := struct {
			ID      string `json:"id"`
			Picture string `json:"picture"`
		}{
			Picture: "/avatar.png",
		}

		findErr := GetInceptionClient(ctx).FindOne("users", inceptiondb.FindQuery{
			Index: "by handle",
			Value: userHandle,
		}, &user)
		if findErr == io.EOF {
			// todo: return page "user not found 404"
			// todo: return
		}
		if findErr != nil {
			// todo: return page "something went wrong"
			// todo: return
		}

		reader, err := GetInceptionClient(ctx).Find("user_honks", inceptiondb.FindQuery{
			Index: "by user-timestamp",
			Skip:  0,
			Limit: 20,
			From: JSON{
				"id":        "",
				"timestamp": 99999999999999,
				"user_id":   user.ID,
			},
			To: JSON{
				"id":        "",
				"timestamp": 0,
				"user_id":   user.ID,
			},
		})
		if err != nil {
			err = fmt.Errorf("error reading from persistence layer")
		}

		honks := []JSON{}

		j := json.NewDecoder(reader)
		for {
			item := struct {
				Honk JSON `json:"honk"`
			}{}
			err := j.Decode(&item)
			if err == io.EOF {
				break
			}
			if err != nil {
				err = fmt.Errorf("error decoding %w", err)
			}
			honks = append(honks, item.Honk)
		}

		description := "@" + userHandle
		if len(honks) > 0 {
			description += ": " + honks[0]["message"].(string)
		}

		title := "Goose @" + userHandle

		template.Execute(w, map[string]interface{}{
			"title":          title,
			"description":    description,
			"name":           userHandle,
			"avatar":         user.Picture,
			"tweets":         honks,
			"og_title":       title,
			"og_url":         selfUrl,
			"og_image":       user.Picture,
			"og_description": description,
		})
	}
}
