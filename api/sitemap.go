package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"goose/inceptiondb"
)

func sitemap(ctx context.Context, w http.ResponseWriter) {

	// todo: this is a naive implementation
	// todo: - url host is hardcoded
	// todo: - xml should be valid (generated properly with some unmarshal/serializer)
	// todo: - loc should be a valid url (escape properly)

	honks := []*Tweet{}
	{
		max := 1000
		reader, err := GetInceptionClient(ctx).Find("honks", inceptiondb.FindQuery{
			Limit: max,
		})
		if err != nil {
			err = fmt.Errorf("error reading from persistence layer")
		}
		j := json.NewDecoder(reader)
		for {
			honk := &Tweet{}
			err := j.Decode(&honk)
			if err == io.EOF {
				break
			}
			if err != nil {
				err = fmt.Errorf("error decoding %w", err)
			}
			honks = append(honks, honk)
		}
	}

	userIDs := map[string]int64{}
	latestTimestamp := int64(0)

	{
		max := 1000
		reader, err := GetInceptionClient(ctx).Find("tweets", inceptiondb.FindQuery{
			Index:   "by timestamp-id",
			Limit:   max,
			Reverse: true,
		})
		if err != nil {
			err = fmt.Errorf("error reading from persistence layer")
		}

		j := json.NewDecoder(reader)
		for {
			honk := &Tweet{}
			err := j.Decode(&honk)
			if err == io.EOF {
				break
			}
			if err != nil {
				err = fmt.Errorf("error decoding %w", err)
			}

			if honk.Timestamp > latestTimestamp {
				latestTimestamp = honk.Timestamp
			}

			if _, exists := userIDs[honk.Nick]; !exists {
				userIDs[honk.Nick] = honk.Timestamp
			}
		}
	}

	w.Header().Set("content-type", "text/xml; charset=UTF-8")

	// Begin XML
	w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.google.com/schemas/sitemap/0.9">
`))

	// Mainstream
	w.Write([]byte(`    <url>
        <loc>https://goose.blue/</loc>
        <lastmod>` + time.Unix(latestTimestamp, 0).Format("2006-01-02") + `</lastmod>
        <changefreq>always</changefreq>
        <priority>1</priority>
    </url>
`))

	// User pages
	for userID, timestamp_unix := range userIDs {

		timestamp := time.Unix(timestamp_unix, 0)

		w.Write([]byte(`    <url>
        <loc>https://goose.blue/user/` + userID + `</loc>
        <lastmod>` + timestamp.Format("2006-01-02") + `</lastmod>
        <changefreq>daily</changefreq>
        <priority>0.7</priority>
    </url>
`))
	}

	// Tweet pages
	for _, honk := range honks {
		w.Write([]byte(`    <url>
        <loc>https://goose.blue/user/` + honk.Nick + `/honk/` + honk.ID + `</loc>
        <lastmod>` + time.Unix(honk.Timestamp, 0).Format("2006-01-02") + `</lastmod>
        <changefreq>weekly</changefreq>
        <priority>0.4</priority>
    </url>
`))

	}

	// End XML
	w.Write([]byte(`</urlset>`))

}
