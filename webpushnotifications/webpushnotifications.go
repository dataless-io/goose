package webpushnotifications

import (
	"io"
	"log"

	"github.com/SherClockHolmes/webpush-go"

	"goose/inceptiondb"
)

// ref: https://datatracker.ietf.org/doc/html/rfc8291

type Config struct {
	PublicKey  string `json:"public_key" usage:"VAPI public key"`
	PrivateKey string `json:"private_key" usage:"VAPI private key"`
}

type Notificator struct {
	Config Config
	Db     *inceptiondb.Client
}

func New(config Config, db *inceptiondb.Client) *Notificator {
	return &Notificator{
		Config: config,
		Db:     db,
	}
}

func (w *Notificator) Send(userId, message string) error {

	userNotification := struct {
		UserId        string                 `json:"user_id"`
		Subscriptions []webpush.Subscription `json:"subscriptions"`
	}{}

	query := inceptiondb.FindQuery{
		Index: "by user_id",
		Value: userId,
	}

	err := w.Db.FindOne("users_webpush", query, &userNotification)
	if err == io.EOF {
		return nil
	}

	for _, subscription := range userNotification.Subscriptions {
		// Send Notification
		func() {
			resp, err := webpush.SendNotification([]byte(message), &subscription, &webpush.Options{
				Subscriber:      "example@example.com", // Do not include "mailto:" // todo: what is this??
				VAPIDPublicKey:  w.Config.PublicKey,
				VAPIDPrivateKey: w.Config.PrivateKey,
				TTL:             30, // todo: hardcoded!
			})
			if err != nil {
				log.Println("WARNING: send notification to '"+userId+"' error:", err.Error())
				return
			}
			defer resp.Body.Close()
		}()

	}

	return nil
}

/*

import (
	"encoding/json"

	webpush "github.com/SherClockHolmes/webpush-go"
)

const (
	subscription    = `{"endpoint":"https://fcm.googleapis.com/fcm/send/eMquXBMOVTQ:APA91bHyK_PTcYyeweJA-PR4Gggcf0T9IwedzqESz4jnOtdPMNjmBSr3LUj3UPDYT3XVOk7PXiSCuxaoYwJ4NZa7V_6N9SryjG9zRZ4sVyUnMxVAJ_JzgI5CJoGt9zJR74B8xqcogDWC","expirationTime":null,"keys":{"p256dh":"BLHIvKt9w2D6Q9pxFOKul1Vhb9MwbpwA6bQkHgCfCPUArRvvtEh-bVXdqYGwE_KQYiKQHkF27Wtd81DXItyoDrs","auth":"FjhBsnOnDpoooXilqP05LQ"}}`
	vapidPublicKey  = "BNu5WieoJqS6vk8U0srRtHFk45zikM1FHKYQf0wHHQGtbnJ2p9oIOY5njfr976WJlpA7ro3PdxAz3WVoNvUNCsk"
	vapidPrivateKey = "JcP38s7uinRMR-mleSS2YvCyRqPjQ_rtzLNBaQNWWE4"
)

func main() {
	// Decode subscription
	s := &webpush.Subscription{}
	json.Unmarshal([]byte(subscription), s)

	// Send Notification
	resp, err := webpush.SendNotification([]byte("Test"), s, &webpush.Options{
		Subscriber:      "example@example.com", // Do not include "mailto:"
		VAPIDPublicKey:  vapidPublicKey,
		VAPIDPrivateKey: vapidPrivateKey,
		TTL:             30,
	})
	if err != nil {
		panic(err.Error())
	}
	defer resp.Body.Close()
}

*/
