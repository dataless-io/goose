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

type ReplyInput struct {
	HonkId  string `json:"honkId"`
	Message string `json:"message"`
}

func Reply(ctx context.Context, input *ReplyInput) (interface{}, error) {

	l := utf8.RuneCountInString(input.Message)
	lmax := 300
	if l > lmax {
		return nil, fmt.Errorf("message length exceeded (%d of %d chars)", l, lmax)
	}
	lmin := 1
	if l < lmin {
		return nil, fmt.Errorf("minimum message length is %d chars", lmin)
	}

	auth := glueauth.GetAuth(ctx)

	inception := GetInceptionClient(ctx)

	parentTweet := &Tweet{}
	query := inceptiondb.FindQuery{
		Index: "by id",
		Value: input.HonkId,
	}
	err := inception.FindOne("tweets", query, &parentTweet)
	if err != nil {
		return nil, err
	}

	parentTweet.LinkedTweet = nil

	tweet := &Tweet{
		ID:          uuid.New().String(),
		Message:     input.Message,
		Timestamp:   time.Now().Unix(),
		UserID:      auth.User.ID,
		Nick:        auth.User.Nick,
		Picture:     auth.User.Picture,
		LinkedTweet: parentTweet,
	}

	inception.Insert("tweets", tweet)
	if err != nil {
		log.Println("Publish:", err.Error())
		return nil, fmt.Errorf("persistence write error")
	}

	tweet.LinkedTweet = nil

	if parentTweet.UserID != tweet.UserID {
		parentTweet.LinkedTweet = tweet
		inception.Insert("tweets", parentTweet)
		if err != nil {
			log.Println("Publish:", err.Error())
			return nil, fmt.Errorf("persistence write error")
		}
	}

	return tweet, nil
}
