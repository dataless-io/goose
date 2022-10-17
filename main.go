package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/fulldump/box"
	"github.com/google/uuid"

	"goose/glueauth"
	"goose/statics"
)

func main() {

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
			box.Get(statics.ServeStatics("")).WithName("serveStatics"),
		)

	b.ListenAndServe()
}

type PublishInput struct {
	Message string `json:"message"`
}

/*
	curl https://inceptiondb.io/collections/tweets -d '{
	  "name": "yoy"
	}'
*/
func Publish(ctx context.Context, input *PublishInput) (interface{}, error) {

	auth := glueauth.GetAuth(ctx)

	tweet := struct {
		ID        string `json:"id"`
		Message   string `json:"message"`
		Timestamp int64  `json:"timestamp"`
		UserID    string `json:"user_id"`
	}{
		ID:        uuid.New().String(),
		Message:   input.Message,
		Timestamp: time.Now().Unix(),
		UserID:    auth.User.ID,
	}

	payload, _ := json.Marshal(tweet) // todo: handle err

	endpoint := "https://inceptiondb.io/collections/tweets"
	req, _ := http.NewRequest("POST", endpoint, bytes.NewReader(payload))
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

	filterByUserID := JSON{
		"user_id": box.GetUrlParameter(ctx, "user-id"),
	}
	data, _ := json.Marshal(filterByUserID)

	params := url.Values{}
	params.Add("skip", "0")
	params.Add("limit", "100")
	params.Add("filter", string(data))

	endpoint := "https://inceptiondb.io/collections/tweets?" + params.Encode()
	req, _ := http.NewRequest("GET", endpoint, nil)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("persistence read error")
	}

	// todo: check this: res.StatusCode

	io.Copy(w, res.Body)

	return nil
}

type JSON = map[string]interface{}
