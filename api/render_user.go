package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/fulldump/box"

	"goose/glueauth"
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
			ID            string `json:"id"`
			Picture       string `json:"picture"`
			JoinTimestamp int64  `json:"join_timestamp"`
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

		// fetch followers
		followers := map[string]bool{}
		auth := glueauth.GetAuth(ctx)
		if auth != nil {

			followerID := auth.User.ID
			// followerID := "user-123" // TODO: remove this

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

		description := "@" + userHandle
		if len(honks) > 0 {
			description += ": " + honks[0]["message"].(string)
		}

		title := "Goose @" + userHandle

		joinDate := time.Unix(0, user.JoinTimestamp)
		joinPretty := joinDate.Format("2006 01 02")

		template.Execute(w, map[string]interface{}{
			"title":          title,
			"description":    description,
			"user":           user,
			"name":           userHandle,
			"avatar":         user.Picture,
			"join_pretty":    joinPretty,
			"tweets":         honks,
			"followers":      followers,
			"og_title":       title,
			"og_url":         selfUrl,
			"og_image":       user.Picture,
			"og_description": description,
		})
	}
}
