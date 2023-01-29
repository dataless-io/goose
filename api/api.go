package api

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/fulldump/box"

	"goose/glueauth"
	"goose/inceptiondb"
	"goose/statics"
	"goose/streams"
)

func Build(inception *inceptiondb.Client, st *streams.Streams, staticsDir string) *box.B {

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

	user.Resource("/follow").
		WithInterceptors(
			glueauth.Require,
		).
		WithActions(
			box.Post(follow),
			box.Delete(unfollow),
		)

	b.Resource("/").
		WithInterceptors(ensureUser).
		WithActions(
			box.Get(renderHome(staticsDir)),
		)

	b.Resource("/user/{user-handle}").
		WithActions(
			box.Get(renderUser(staticsDir)),
		)

	b.Resource("/user/{user-handle}/honk/{honk-id}").
		WithActions(
			box.Get(renderHonk(staticsDir)),
		)

	b.Resource("/sitemap.xml").
		WithActions(
			box.Get(sitemap),
		)

	// Mount statics
	b.Resource("/*").
		WithInterceptors(
			IfModifiedSince(),
		).
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
			ID            string `json:"id"`
			Handle        string `json:"handle"`
			Nick          string `json:"nick"`
			Picture       string `json:"picture"`
			JoinTimestamp int64  `json:"join_timestamp"`
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
			user.JoinTimestamp = time.Now().UnixNano()
			insertErr := inception.Insert("users", user)
			if insertErr != nil {
				fmt.Println("ERROR: insert user:", insertErr.Error())
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

	t = template.New(name).Funcs(map[string]any{
		"json": func(input any) string {
			result, err := json.Marshal(input)
			if err != nil {
				return "" // todo: log or do somehting with this?
			}
			return string(result)
		},
	})

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
