package glueauth

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/fulldump/box"
)

const XGlueAuthentication = "X-Glue-Authentication"

type GlueAuthentication struct {
	Session struct {
		ID string `json:"id"`
	} `json:"session"`
	User struct {
		ID      string `json:"id"`
		Nick    string `json:"nick"`
		Picture string `json:"picture"`
		Email   string `json:"email"`
	} `json:"user"`
}

var ErrUnauthorized = errors.New("unauthorized")

func Require(next box.H) box.H {
	return func(ctx context.Context) {

		d := box.GetRequest(ctx).Header.Get(XGlueAuthentication)

		if d == "" {
			box.SetError(ctx, ErrUnauthorized)
			return
		}

		a := &GlueAuthentication{}

		err := json.Unmarshal([]byte(d), &a)
		if err != nil {
			box.SetError(ctx, ErrUnauthorized)
			return
		}

		ctx = SetAuth(ctx, a)

		next(ctx)
	}
}

const key = "6fbc299a-3546-11ed-bf91-87a0b0cea4af"

func SetAuth(ctx context.Context, a *GlueAuthentication) context.Context {
	return context.WithValue(ctx, key, a)
}

func GetAuth(ctx context.Context) *GlueAuthentication {
	v := ctx.Value(key)
	return v.(*GlueAuthentication)
}
