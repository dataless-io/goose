package api

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"

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

	user.Resource("/follow").
		WithInterceptors(
			glueauth.Require,
		).
		WithActions(
			box.Post(func(ctx context.Context) error {

				userID := box.GetUrlParameter(ctx, "user-id")

				user := JSON{}
				findErr := GetInceptionClient(ctx).FindOne("users", inceptiondb.FindQuery{
					Index: "by id",
					Value: userID,
				}, &user)
				if findErr == io.EOF {
					// todo: return page "user not found 404"
					box.GetResponse(ctx).WriteHeader(http.StatusNotFound)
					return errors.New("user does not exist")
				}
				if findErr != nil {
					// todo: return page "something went wrong"
					return errors.New("unexpected persistence error")
				}

				me := glueauth.GetAuth(ctx)

				err := GetInceptionClient(ctx).Insert("followers", JSON{
					"user_id":     userID,
					"follower_id": me.User.ID,
					"user":        user,
				})
				if err == inceptiondb.ErrorAlreadyExist {
					return nil
				}
				return err
			}),
			box.Delete(func(ctx context.Context) interface{} {

				userID := box.GetUrlParameter(ctx, "user-id")
				me := glueauth.GetAuth(ctx)

				removed, err := GetInceptionClient(ctx).Remove("followers", inceptiondb.FindQuery{
					Index: "by follower",
					Limit: 100, // TODO: this max number of followers...
					From: JSON{
						"follower_id": me.User.ID,
						"user_id":     userID,
					},
					To: JSON{
						"follower_id": me.User.ID,
						"user_id":     userID + "zzzzzzzzzzzz",
					},
				})

				io.Copy(os.Stdout, removed)

				return err
			}),
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
