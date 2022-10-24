package api

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"goose/inceptiondb"
)

func MainStream(ctx context.Context, w http.ResponseWriter) error {

	collectionInfo, err := GetInceptionClient(ctx).GetCollection("tweets")
	if err != nil {
		return fmt.Errorf("persistence read error")
	}

	max := 100
	skip := collectionInfo.Total - max
	if skip < 0 {
		skip = 0
	}

	reader, err := GetInceptionClient(ctx).Find("tweets", inceptiondb.FindQuery{
		Skip:  skip,
		Limit: max,
	})
	if err != nil {
		return fmt.Errorf("error reading from persistence layer")
	}

	io.Copy(w, reader)

	return nil
}
