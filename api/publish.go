package api

import (
	"context"
	"fmt"
	"log"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"goose/glueauth"
	"goose/inceptiondb"
)

type PublishInput struct {
	Message      string `json:"message"`
	ParentHonkID string `json:"parent_honk_id"`
}

/*
	curl https://inceptiondb.io/collections/tweets -d '{
	  "name": "yoy"
	}'
*/
type Tweet struct {
	ID          string `json:"id"`
	Message     string `json:"message"`
	Timestamp   int64  `json:"timestamp"`
	UserID      string `json:"user_id"`
	Nick        string `json:"nick"`
	Picture     string `json:"picture"`
	LinkedTweet *Tweet `json:"linked_tweet,omitempty"`
}

func Publish(ctx context.Context, input *PublishInput) (interface{}, error) {

	// validate user input
	l := utf8.RuneCountInString(input.Message)
	lmax := 300
	if l > lmax {
		return nil, fmt.Errorf("message length exceeded (%d of %d chars)", l, lmax)
	}
	lmin := 1
	if l < lmin {
		return nil, fmt.Errorf("minimum message length is %d chars", lmin)
	}

	// Check hook id
	var parentTweet *Tweet
	if input.ParentHonkID != "" {
		query := inceptiondb.FindQuery{
			Index: "by id",
			Value: input.ParentHonkID,
		}
		err := GetInceptionClient(ctx).FindOne("tweets", query, &parentTweet)
		if err != nil {
			return nil, fmt.Errorf("parent_honk_id '%s' not found", input.ParentHonkID)
		}
	}

	auth := glueauth.GetAuth(ctx)

	tweet := Tweet{
		ID:          uuid.New().String(),
		Message:     input.Message,
		Timestamp:   time.Now().Unix(),
		UserID:      auth.User.ID,
		Nick:        auth.User.Nick,
		Picture:     auth.User.Picture,
		LinkedTweet: parentTweet,
	}

	{
		err := GetStreams(ctx).Send("honk_create", tweet)
		if err != nil {
			log.Println("Publish:", err.Error())
			return nil, fmt.Errorf("persistence write error")
		}
	}

	{
		err := GetInceptionClient(ctx).Insert("tweets", tweet)
		if err != nil {
			log.Println("Publish:", err.Error())
			return nil, fmt.Errorf("persistence write error")
		}
	}

	return tweet, nil
}
