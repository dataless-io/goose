package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/fulldump/box"
)

func IfModifiedSince() box.I {

	deployTime := time.Now().UTC()
	lastModified := deployTime.Format(time.RFC1123)
	fmt.Println("Last-Modified:", lastModified)

	return func(next box.H) box.H {
		return func(ctx context.Context) {

			r := box.GetRequest(ctx)
			ifModifiedSince := r.Header.Get(`If-Modified-Since`)
			if ifModifiedSince != "" {
				t, err := time.Parse(time.RFC1123, ifModifiedSince) // todo: could be time.RFC850 ??
				fmt.Println(deployTime, t)
				if err == nil && t.Before(deployTime) {
					box.GetResponse(ctx).WriteHeader(http.StatusNotModified)
					return
				}
			}

			w := box.GetResponse(ctx)
			w.Header().Set("Last-Modified", lastModified)
			next(ctx)
		}
	}
}
