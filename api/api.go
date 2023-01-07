package api

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/fulldump/box"

	"goose/glueauth"
	"goose/inceptiondb"
	"goose/statics"
	"goose/streams"
)

func Build(inception *inceptiondb.Client, st *streams.Streams, staticsDir string) http.Handler {

	b := box.NewBox()

	b.WithInterceptors(
		InjectInceptionClient(inception),
		InjectStreams(st),
		glueauth.Auth,
	)

	b.WithInterceptors(PrettyErrorInterceptor)

	b.WithInterceptors(func(next box.H) box.H {
		return func(ctx context.Context) {
			req := box.GetRequest(ctx)
			log.Println(req.Method, req.URL.String())
			next(ctx)
		}
	})

	v0 := b.Resource("/v0")

	v0.Resource("/publish").WithActions(
		box.Post(Publish),
	).WithInterceptors(
		glueauth.Require,
	)

	// todo: deprecate
	v0.Resource("/reply").WithActions(
		box.Post(Reply),
	).WithInterceptors(
		glueauth.Require,
	)

	user := v0.Resource("/users/{user-id}")

	user.
		WithActions(
			box.Get(func(w http.ResponseWriter, r *http.Request) {

			}),
		)

	user.Resource("/timeline").
		WithActions(
			box.Get(Timeline),
		)

	user.Resource("/mainstream").
		WithActions(
			box.Get(MainStream),
		)

	user.Resource("/followers").WithActions(
		box.Get(func() interface{} {
			return "followers"
		}),
	)

	user.Resource("/following").WithActions(
		box.Get(func() interface{} {
			return "following"
		}),
	)

	beta := b

	t_home, err := t("home", staticsDir, "pages/template.gohtml", "pages/home.gohtml")
	if err != nil {
		panic(err)
	}

	beta.Resource("/").
		WithInterceptors(ensureUser).
		WithActions(
			box.Get(func(ctx context.Context, w http.ResponseWriter) {

				w.Header().Set(`Link`, `<https://goose.blue/>; rel="canonical"`)

				max := 100
				reader, err := GetInceptionClient(ctx).Find("tweets", inceptiondb.FindQuery{
					Index:   "by timestamp-id",
					Limit:   max,
					Reverse: true,
				})
				if err != nil {
					err = fmt.Errorf("error reading from persistence layer")
				}

				tweets := []JSON{}

				j := json.NewDecoder(reader)
				for {
					tweet := JSON{}
					err := j.Decode(&tweet)
					if err == io.EOF {
						break
					}
					if err != nil {
						err = fmt.Errorf("error decoding %w", err)
					}
					tweets = append(tweets, tweet)
				}

				t_home.Execute(w, map[string]interface{}{
					"title":  "Home page",
					"name":   "Fulanezxxx",
					"tweets": tweets,
				})

			}),
		)

	beta.Resource("/sitemap.xml").
		WithActions(
			box.Get(func(ctx context.Context, w http.ResponseWriter) {

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
        <changefreq>daily</changefreq>
        <priority>0.8</priority>
    </url>
`))

				// User pages
				for userID, timestamp_unix := range userIDs {

					timestamp := time.Unix(timestamp_unix, 0)

					w.Write([]byte(`    <url>
        <loc>https://goose.blue/user/` + userID + `</loc>
        <lastmod>` + timestamp.Format("2006-01-02") + `</lastmod>
        <changefreq>daily</changefreq>
        <priority>0.8</priority>
    </url>
`))
				}

				// Tweet pages
				for _, honk := range honks {
					w.Write([]byte(`    <url>
        <loc>https://goose.blue/user/` + honk.Nick + `/honk/` + honk.ID + `</loc>
        <lastmod>` + time.Unix(honk.Timestamp, 0).Format("2006-01-02") + `</lastmod>
        <changefreq>daily</changefreq>
        <priority>0.6</priority>
    </url>
`))

				}

				// End XML
				w.Write([]byte(`</urlset>`))

			}),
		)

	t_user, err := t("home", staticsDir, "pages/template.gohtml", "pages/user.gohtml")
	if err != nil {
		panic(err)
	}

	// todo: rename to user-handle:
	beta.Resource("/user/{user-id}").
		WithActions(
			box.Get(func(ctx context.Context, w http.ResponseWriter) {

				userHandle := box.GetUrlParameter(ctx, "user-id")

				w.Header().Set(`Link`, `<https://goose.blue/user/`+url.PathEscape(userHandle)+`>; rel="canonical"`)

				user := struct {
					ID      string `json:"id"`
					Picture string `json:"picture"`
				}{
					Picture: "/avatar.png",
				}

				findErr := inception.FindOne("users", inceptiondb.FindQuery{
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
					Limit: 100,
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

				t_user.Execute(w, map[string]interface{}{
					"title":  "Home page",
					"name":   userHandle,
					"avatar": user.Picture,
					"tweets": honks,
				})

			}),
		)

	t_article, err := t("article", staticsDir, "pages/template.gohtml", "pages/article.gohtml")
	if err != nil {
		panic(err)
	}

	// todo: rename to user-handle:
	beta.Resource("/user/{user-id}/honk/{honk-id}").
		WithActions(
			box.Get(func(ctx context.Context, w http.ResponseWriter) {

				userHandle := box.GetUrlParameter(ctx, "user-id")
				honkId := box.GetUrlParameter(ctx, "honk-id")

				w.Header().Set(`Link`, `<https://goose.blue/user/`+url.PathEscape(userHandle)+`/honk/+`+url.PathEscape(honkId)+`>; rel="canonical"`)

				var honk Tweet
				findErr := inception.FindOne("honks", inceptiondb.FindQuery{
					Index: "by id",
					Value: honkId,
				}, &honk)
				if findErr != nil {
					// todo: render a pretty and user friendly error page
					w.WriteHeader(http.StatusNotFound)
					return
				}

				if honk.Nick != userHandle {
					// todo: render a pretty and user friendly error page
					w.WriteHeader(http.StatusNotFound)
					return
				}

				t_article.Execute(w, map[string]interface{}{
					"title": "Honk page",
					"name":  userHandle,
					"honk":  honk,
				})

			}),
		)

	// Mount statics
	b.Resource("/*").
		WithActions(
			box.Get(statics.ServeStatics(staticsDir)).WithName("serveStatics"),
		)

	return b
}

func ensureUser(next box.H) box.H {
	return func(ctx context.Context) {

		next(ctx)

		auth := glueauth.GetAuth(ctx)
		if auth == nil {
			return
		}

		user := struct {
			ID      string `json:"id"`
			Handle  string `json:"handle"`
			Nick    string `json:"nick"`
			Picture string `json:"picture"`
		}{}

		inception := GetInceptionClient(ctx)
		err := inception.FindOne("users", inceptiondb.FindQuery{
			Index: "by id",
			Value: auth.User.ID,
		}, &user)
		if err == io.EOF {
			user.ID = auth.User.ID
			user.Handle = auth.User.Nick // todo: conflict with handler?
			user.Nick = auth.User.Nick
			user.Picture = auth.User.Picture
			insertErr := inception.Insert("users", user)
			if insertErr != nil {
				fmt.Println("ERROR: insert user:", err.Error())
			}
			return
		}
		if err != nil {
			fmt.Println("ERROR: find user:", err.Error())
		}
	}
}

type JSON = map[string]interface{}

func t(name, staticsDir string, filenames ...string) (t *template.Template, err error) {

	f := statics.FileReader(staticsDir)

	t = template.New(name)

	for _, filename := range filenames {

		data, err := f(filename)
		if err != nil {
			return nil, err
		}

		t, err = t.Parse(string(data))
		if err != nil {
			return nil, err
		}
	}

	return
}
