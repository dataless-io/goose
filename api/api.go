package api

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"

	"github.com/fulldump/box"

	"goose/glueauth"
	"goose/inceptiondb"
	"goose/statics"
)

func Build(inception *inceptiondb.Client, staticsDir string) http.Handler {

	b := box.NewBox()

	b.WithInterceptors(
		InjectInceptionClient(inception),
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

	beta := b.Resource("/beta")

	t_home, err := t("home", "./statics/www/", "pages/template.gohtml", "pages/home.gohtml")
	if err != nil {
		panic(err)
	}

	beta.Resource("/").
		WithActions(
			box.Get(func(ctx context.Context, w http.ResponseWriter) {

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

	t_user, err := t("home", "./statics/www/", "pages/template.gohtml", "pages/user.gohtml")
	if err != nil {
		panic(err)
	}

	beta.Resource("/user/{user-id}").
		WithActions(
			box.Get(func(ctx context.Context, w http.ResponseWriter) {

				userId := box.GetUrlParameter(ctx, "user-id")

				reader, err := GetInceptionClient(ctx).Find("tweets", inceptiondb.FindQuery{
					Index: "by user-timestamp-id",
					Skip:  0,
					Limit: 100,
					From: JSON{
						"id":        "",
						"timestamp": 99999999999999,
						"user_id":   userId,
					},
					To: JSON{
						"id":        "",
						"timestamp": 0,
						"user_id":   userId,
					},
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

				t_user.Execute(w, map[string]interface{}{
					"title":  "Home page",
					"name":   userId,
					"tweets": tweets,
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
