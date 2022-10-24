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

	err := inception.EnsureCollection("tweets")
	if err != nil {
		panic("ensure collection tweets: " + err.Error())
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
