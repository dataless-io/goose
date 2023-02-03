package api

import (
	"context"
	"errors"
	"io"
	"net/http"

	"github.com/fulldump/box"

	"goose/glueauth"
	"goose/inceptiondb"
	"goose/webpushnotifications"
)

func follow(notificator *webpushnotifications.Notificator) any {
	return func(ctx context.Context) error {

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
		if err != nil {
			// do something
			return err
		}

		notificator.Send(userID, webpushnotifications.Options{
			Title: "@" + me.User.Nick + " te está siguiendo",
			Icon:  me.User.Picture,
			Data: map[string]interface{}{
				"open": "https://goose.blue/user/" + me.User.Nick,
			},
		})

		return nil
	}
}
