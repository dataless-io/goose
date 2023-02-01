package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"

	"goose/api"
	"goose/inceptiondb"
	"goose/streams"
	"goose/webpushnotifications"
)

func Bootstrap(c Config) (start, stop func() error) {

	inception := inceptiondb.NewClient(c.Inception)

	{
		err := inception.EnsureIndex("users", &inceptiondb.IndexOptions{
			Name:   "by id",
			Type:   "map",
			Field:  "id",
			Sparse: false,
		})
		if err != nil {
			panic("ensure index 'by id' on users: " + err.Error())
		}
	}
	{
		err := inception.EnsureIndex("users_webpush", &inceptiondb.IndexOptions{
			Name:   "by user_id",
			Type:   "map",
			Field:  "user_id",
			Sparse: false,
		})
		if err != nil {
			panic("ensure index 'by user_id' on users_webpush: " + err.Error())
		}
	}
	{
		err := inception.EnsureIndex("users", &inceptiondb.IndexOptions{
			Name:   "by handle",
			Type:   "map",
			Field:  "handle",
			Sparse: false,
		})
		if err != nil {
			panic("ensure index 'by handle' on users: " + err.Error())
		}
	}
	{
		err := inception.EnsureIndex("tweets", &inceptiondb.IndexOptions{
			Name:   "by timestamp-id",
			Type:   "btree",
			Fields: []string{"timestamp", "id"},
			Sparse: true,
		})
		if err != nil {
			panic("ensure index 'by timestamp-id' on tweets: " + err.Error())
		}
	}
	{
		err := inception.EnsureIndex("tweets", &inceptiondb.IndexOptions{
			Name:   "by user-timestamp-id",
			Type:   "btree",
			Fields: []string{"user_id", "-timestamp", "id"},
			Sparse: true,
		})
		if err != nil {
			panic("ensure index 'by timestamp-id' on tweets: " + err.Error())
		}
	}
	{
		err := inception.EnsureIndex("tweets", &inceptiondb.IndexOptions{
			Name:   "by id",
			Type:   "map",
			Field:  "id",
			Sparse: true,
		})
		if err != nil {
			panic("ensure index 'by id' on tweets: " + err.Error())
		}
	}
	{
		err := inception.EnsureIndex("user_honks", &inceptiondb.IndexOptions{
			Name:   "by user-timestamp",
			Type:   "btree",
			Fields: []string{"user_id", "-timestamp"},
			Sparse: true,
		})
		if err != nil {
			panic("ensure index 'by user-timestamp' on user_honks: " + err.Error())
		}
	}
	{
		err := inception.EnsureIndex("honks", &inceptiondb.IndexOptions{
			Name:  "by id",
			Type:  "map",
			Field: "id",
		})
		if err != nil {
			panic("ensure index 'by user-timestamp' on user_honks: " + err.Error())
		}
	}
	{
		err := inception.EnsureIndex("followers", &inceptiondb.IndexOptions{
			Name:   "by user",
			Type:   "btree",
			Fields: []string{"user_id", "follower_id"},
			Sparse: true,
		})
		if err != nil {
			panic("ensure index 'by user' on followers: " + err.Error())
		}
	}
	{
		err := inception.EnsureIndex("followers", &inceptiondb.IndexOptions{
			Name:   "by follower",
			Type:   "btree",
			Fields: []string{"follower_id", "user_id"},
			Sparse: true,
		})
		if err != nil {
			panic("ensure index 'by follower' on followers: " + err.Error())
		}
	}

	st := streams.NewStreams(inception)

	{
		err := st.Ensure("honk_create")
		if err != nil {
			panic("ensure stream 'honk_create':" + err.Error())
		}
	}
	{
		err := st.Ensure("user_follow")
		if err != nil {
			panic("ensure stream 'user_follow':" + err.Error())
		}
	}

	notificator := webpushnotifications.New(c.WebPush, inception)

	go func() {
		err := st.Receive("honk_create", "insert_honk", func(data []byte) error {
			honk := api.Tweet{}
			json.Unmarshal(data, &honk)

			err := inception.Insert("honks", honk)
			if err == inceptiondb.ErrorAlreadyExist {
				return nil
			}
			if err != nil {
				log.Println("ERROR: mention:", honk.UserID, err.Error())
			}
			return nil
		})
		if err != nil {
			panic("stream receive 'honk_create'->'insert_honk':" + err.Error())
		}
	}()

	go func() {
		err := st.Receive("honk_create", "insert_user_honk", func(data []byte) error {
			honk := api.Tweet{}
			json.Unmarshal(data, &honk)
			err := inception.Insert("user_honks", api.JSON{
				"user_id":   honk.UserID,
				"timestamp": honk.Timestamp,
				"honk":      honk,
			})
			if err == inceptiondb.ErrorAlreadyExist {
				return nil
			}
			if err != nil {
				log.Println("ERROR: user_honks:", honk.UserID, err.Error())
			}
			return nil
		})
		if err != nil {
			panic("stream receive 'honk_create'->'insert_user_honk':" + err.Error())
		}
	}()

	go func() {
		err := st.Receive("honk_create", "mention", func(data []byte) error {
			honk := api.Tweet{}
			json.Unmarshal(data, &honk)

			mentions := findMentions(honk.Message)
			for _, mention := range mentions {
				user := struct {
					ID string `json:"id"`
				}{}
				findErr := inception.FindOne("users", inceptiondb.FindQuery{
					Index: "by handle",
					Value: mention,
				}, &user)
				if findErr != nil {
					continue
				}
				err := inception.Insert("user_honks", api.JSON{
					"user_id":   user.ID,
					"timestamp": honk.Timestamp,
					"honk":      honk,
				})
				if err != nil {
					log.Println("ERROR: mention:", user.ID, err.Error())
				}
				notificator.Send(user.ID, honk.Nick+" dice: "+honk.Message)
			}

			return nil
		})
		if err != nil {
			panic("stream receive 'honk_create'->'mention':" + err.Error())
		}
	}()

	go func() {
		err := st.Receive("honk_create", "follow_user", func(data []byte) error {
			honk := api.Tweet{}
			json.Unmarshal(data, &honk)

			// fetch followers
			reader, err := inception.Find("followers", inceptiondb.FindQuery{
				Index: "by user",
				Limit: 100, // TODO: this max number of followers...
				From: map[string]interface{}{
					"user_id":     honk.UserID,
					"follower_id": "",
				},
				To: map[string]interface{}{
					"user_id":     honk.UserID,
					"follower_id": "z",
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

				err = inception.Insert("user_honks", api.JSON{
					"user_id":   relationship.FollowerID,
					"timestamp": honk.Timestamp,
					"honk":      honk,
				})
				if err != nil {
					log.Println("ERROR: follow:", relationship.FollowerID, err.Error())
				}
				notificator.Send(relationship.FollowerID, honk.Nick+" dice: "+honk.Message)
			}

			return nil
		})
		if err != nil {
			panic("stream receive 'honk_create'->'follow_user':" + err.Error())
		}
	}()

	a := api.Build(inception, st, c.Statics, notificator)

	// Add compression
	if c.EnableCompression {
		fmt.Println("Compression enabled")
		a.WithInterceptors(api.Compression)
	}

	s := &http.Server{
		Addr:    c.Addr,
		Handler: a,
	}
	log.Println("listening on ", s.Addr)

	start = func() error {
		err := s.ListenAndServe()
		return err
	}

	stop = func() error {
		log.Println("Stop streams...")
		st.Close()
		st.Wait()
		log.Println("Stop http server...")
		return s.Shutdown(context.Background())
	}

	return start, stop
}

// TODO: move this to a proper package
func normalizeHandle(a string) string {
	a = strings.TrimPrefix(a, "@")
	a = strings.ToLower(a)
	return a
}

// TODO: move this to a proper package
func unique(items []string) []string {

	m := map[string]bool{}
	for _, item := range items {
		m[item] = true
	}

	result := make([]string, len(m))
	i := 0
	for v := range m {
		result[i] = v
		i++
	}

	return result
}

// TODO: move this to a proper package
var findMentionRegex = regexp.MustCompile(`@[a-zA-Z0-9_]+`)

func findMentions(message string) []string {
	mentions := findMentionRegex.FindAllString(message, -1)

	// normalize
	for i, mention := range mentions {
		mentions[i] = normalizeHandle(mention)
	}

	// unique
	return unique(mentions)
}
