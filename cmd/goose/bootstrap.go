package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"

	"goose/api"
	"goose/inceptiondb"
	"goose/streams"
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

	st := streams.NewStreams(inception)

	{
		err := st.Ensure("honk_create")
		if err != nil {
			panic("ensure stream 'honk_create':" + err.Error())
		}
	}

	go func() {
		err := st.Receive("honk_create", "insert_honk", func(data []byte) error {
			tweet := api.Tweet{}
			json.Unmarshal(data, &tweet)
			return inception.Insert("honks", tweet)
		})
		if err != nil {
			panic("stream receive 'honk_create'->'insert_honk':" + err.Error())
		}
	}()

	go func() {
		err := st.Receive("honk_create", "insert_user_honk", func(data []byte) error {
			honk := api.Tweet{}
			json.Unmarshal(data, &honk)
			insertErr := inception.Insert("user_honks", api.JSON{
				"user_id":   honk.UserID,
				"timestamp": honk.Timestamp,
				"honk":      honk,
			})
			if insertErr != nil {
				log.Println("ERROR: mention:", honk.UserID, insertErr.Error())
			}
			return nil
		})
		if err != nil {
			panic("stream receive 'honk_create'->'insert_honk':" + err.Error())
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
				insertErr := inception.Insert("user_honks", api.JSON{
					"user_id":   user.ID,
					"timestamp": honk.Timestamp,
					"honk":      honk,
				})
				if insertErr != nil {
					log.Println("ERROR: mention:", user.ID, insertErr.Error())
				}
			}

			return nil
		})
		if err != nil {
			panic("stream receive 'honk_create'->'mention':" + err.Error())
		}
	}()

	a := api.Build(inception, st, c.Statics)

	s := &http.Server{
		Addr:    c.Addr,
		Handler: a,
	}
	fmt.Println("listening on ", s.Addr)

	start = func() error {
		err := s.ListenAndServe()
		st.Wait()
		return err
	}

	stop = func() error {
		st.Close()
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
