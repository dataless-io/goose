package api

import (
	"context"
	"io"
	"os"

	"github.com/fulldump/box"

	"goose/glueauth"
	"goose/inceptiondb"
)

func unfollow(ctx context.Context) interface{} {

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
}
