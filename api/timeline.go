package api

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/fulldump/box"

	"goose/inceptiondb"
)

func Timeline(ctx context.Context, w http.ResponseWriter) error {

	reader, err := GetInceptionClient(ctx).Find("tweets", inceptiondb.FindQuery{
		Index: "by user-timestamp-id",
		Skip:  0,
		Limit: 100,
		From: JSON{
			"id":        "",
			"timestamp": 99999999999999,
			"user_id":   box.GetUrlParameter(ctx, "user-id"),
		},
		To: JSON{
			"id":        "",
			"timestamp": 0,
			"user_id":   box.GetUrlParameter(ctx, "user-id"),
		},
	})
	if err != nil {
		return fmt.Errorf("error reading from persistence layer")
	}

	io.Copy(w, reader)

	return nil
}
