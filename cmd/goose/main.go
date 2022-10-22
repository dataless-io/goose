package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
	"unicode/utf8"

	"github.com/fulldump/box"
	"github.com/fulldump/goconfig"
	"github.com/google/uuid"

	"goose/glueauth"
	"goose/statics"
)

type Config struct {
	Addr    string `json:"addr"`
	Statics string `json:"statics"`
}

// TODO: inject this and make it configurable
var base = "https://saas.inceptiondb.io/v1"
var databaseID = "ab9965be-56a7-4d55-bf14-3e8b96d742c2"
var apiKey = "3a143a03-2ee4-46e7-8286-64a17ab2b642"
var apiSecret = "cac508a9-32a1-43c0-ba82-859805906972"

/*
databaseID: ab9965be-56a7-4d55-bf14-3e8b96d742c2
apiKey: f22b7f32-9bbc-42c1-97a6-96f0abec655d
apiSecret: 8048a56c-0406-4af3-9a59-a17ca33fe7fa
*/

func main() {

	c := Config{
		Addr: ":8080", // default address
	}
	goconfig.Read(&c)

	ensureCollection("tweets")

	b := box.NewBox()

	b.WithInterceptors(PrettyErrorInterceptor)

	v0 := b.Resource("/v0")

	v0.Resource("/publish").WithActions(
		box.Post(Publish),
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

	// Mount statics
	b.Resource("/*").
		WithActions(
			box.Get(statics.ServeStatics(c.Statics)).WithName("serveStatics"),
		)

	s := &http.Server{
		Addr:    c.Addr,
		Handler: b,
	}
	fmt.Println("listening on ", s.Addr)
	err := s.ListenAndServe()
	if err != nil {
		fmt.Println(err.Error())
	}
}

func ensureCollection(collectionName string) {
	endpoint := base + "/databases/" + databaseID + "/collections"

	payload, err := json.Marshal(JSON{
		"name": collectionName,
	}) // todo: handle err
	if err != nil {
		panic(fmt.Errorf("cannot create collection: '%s'", err.Error()))
	}

	req, _ := http.NewRequest("POST", endpoint, bytes.NewReader(payload))
	req.Header.Set("Api-Key", apiKey)
	req.Header.Set("Api-Secret", apiSecret)

	resp, err := http.DefaultClient.Do(req)

	fmt.Println("Created collection tweets:", resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	fmt.Println("body:", string(body))
}

type PublishInput struct {
	Message string `json:"message"`
}

/*
	curl https://inceptiondb.io/collections/tweets -d '{
	  "name": "yoy"
	}'
*/
type Tweet struct {
	ID        string `json:"id"`
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"`
	UserID    string `json:"user_id"`
	Nick      string `json:"nick"`
	Picture   string `json:"picture"`
}

func Publish(ctx context.Context, input *PublishInput) (interface{}, error) {

	l := utf8.RuneCountInString(input.Message)
	lmax := 300
	if l > lmax {
		return nil, fmt.Errorf("message length exceeded (%d of %d chars)", l, lmax)
	}

	auth := glueauth.GetAuth(ctx)

	tweet := Tweet{
		ID:        uuid.New().String(),
		Message:   input.Message,
		Timestamp: time.Now().Unix(),
		UserID:    auth.User.ID,
		Nick:      auth.User.Nick,
		Picture:   auth.User.Picture,
	}

	payload, _ := json.Marshal(tweet) // todo: handle err

	endpoint := base + "/databases/" + databaseID + "/collections/tweets:insert"
	req, _ := http.NewRequest("POST", endpoint, bytes.NewReader(payload))
	req.Header.Set("Api-Key", apiKey)
	req.Header.Set("Api-Secret", apiSecret)

	// todo: handle err

	_, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("persistence write error")
	}

	// todo: check response

	return tweet, nil
}

func PrettyErrorInterceptor(next box.H) box.H {
	return func(ctx context.Context) {

		next(ctx)

		err := box.GetError(ctx)
		if err == nil {
			return
		}
		w := box.GetResponse(ctx)

		if err == glueauth.ErrUnauthorized {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]interface{}{
					"message":     err.Error(),
					"description": fmt.Sprintf("user is not authenticated"),
				},
			})
			return
		}

		if err == box.ErrResourceNotFound {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]interface{}{
					"message":     err.Error(),
					"description": fmt.Sprintf("resource '%s' not found", box.GetRequest(ctx).URL.String()),
				},
			})
			return
		}

		if err == box.ErrMethodNotAllowed {
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]interface{}{
					"message":     err.Error(),
					"description": fmt.Sprintf("method '%s' not allowed", box.GetRequest(ctx).Method),
				},
			})
			return
		}

		if _, ok := err.(*json.SyntaxError); ok {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]interface{}{
					"message":     err.Error(),
					"description": "Malformed JSON",
				},
			})
			return
		}

		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"message":     err.Error(),
				"description": "Unexpected error",
			},
		})

	}
}

func Timeline(ctx context.Context, w http.ResponseWriter) error {

	payload, err := json.Marshal(JSON{
		"skip":  0,
		"limit": 100,
		"filter": JSON{
			"user_id": box.GetUrlParameter(ctx, "user-id"),
		},
	}) // todo: handle err
	if err != nil {
		return fmt.Errorf("error reading from persistence layer")
	}

	endpoint := base + "/databases/" + databaseID + "/collections/tweets:find"

	req, _ := http.NewRequest("POST", endpoint, bytes.NewReader(payload))
	req.Header.Set("Api-Key", apiKey)
	req.Header.Set("Api-Secret", apiSecret)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("persistence read error")
	}

	// todo: check this: res.StatusCode

	io.Copy(w, res.Body)

	return nil
}

func MainStream(ctx context.Context, w http.ResponseWriter) error {

	collectionTweets := struct {
		Total int
	}{}

	{
		endpoint := base + "/databases/" + databaseID + "/collections/tweets"
		req, _ := http.NewRequest("GET", endpoint, nil)
		req.Header.Set("Api-Key", apiKey)
		req.Header.Set("Api-Secret", apiSecret)
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("persistence read error")
		}
		// io.Copy(os.Stdout, res.Body)
		json.NewDecoder(res.Body).Decode(&collectionTweets) // todo: handle error
	}

	{
		n := 100

		skip := collectionTweets.Total - n
		if skip < 0 {
			skip = 0
		}

		payload, err := json.Marshal(JSON{
			"skip":  skip,
			"limit": n,
		}) // todo: handle err
		if err != nil {
			return fmt.Errorf("error reading from persistence layer")
		}

		endpoint := base + "/databases/" + databaseID + "/collections/tweets:find"

		req, _ := http.NewRequest("POST", endpoint, bytes.NewReader(payload))
		req.Header.Set("Api-Key", apiKey)
		req.Header.Set("Api-Secret", apiSecret)

		res, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("persistence read error")
		}

		// todo: check this: res.StatusCode

		io.Copy(w, res.Body)
	}

	return nil
}

type JSON = map[string]interface{}
