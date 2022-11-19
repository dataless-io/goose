package main

import (
	"context"
	"fmt"
	"net/http"

	"goose/api"
	"goose/inceptiondb"
)

func Bootstrap(c Config) (start, stop func() error) {

	inception := inceptiondb.NewClient(c.Inception)

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

	a := api.Build(inception, c.Statics)

	s := &http.Server{
		Addr:    c.Addr,
		Handler: a,
	}
	fmt.Println("listening on ", s.Addr)

	close := func() error {
		return s.Shutdown(context.Background())
	}

	return s.ListenAndServe, close
}
