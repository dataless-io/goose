package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

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
			tweet := api.Tweet{}
			json.Unmarshal(data, &tweet)
			return inception.Insert("user_honks", api.JSON{
				"user_id":   tweet.UserID,
				"timestamp": tweet.Timestamp,
				"honk":      tweet,
			})
		})
		if err != nil {
			panic("stream receive 'honk_create'->'insert_honk':" + err.Error())
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
