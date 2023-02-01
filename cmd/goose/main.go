package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/SherClockHolmes/webpush-go"
	"github.com/fulldump/goconfig"

	"goose/inceptiondb"
	"goose/webpushnotifications"
)

type Config struct {
	Addr              string `json:"addr"`
	Statics           string `json:"statics"`
	Inception         inceptiondb.Config
	EnableCompression bool                        `json:"enable_compression"`
	WebPush           webpushnotifications.Config `json:"web_push"`
}

func main() {

	c := Config{
		Addr: ":8080", // default address
		Inception: inceptiondb.Config{
			Base: "https://saas.inceptiondb.io/v1",

			// Development credentials by default:
			DatabaseID: "1b5b8fef-db19-4308-852d-d0c61eda7143",
			ApiKey:     "973417a6-ea20-4ac9-8b9f-6f3db9213a01",
			ApiSecret:  "ebac1daf-48fc-4e88-a6ba-04ebf1b48125",
		},
	}
	goconfig.Read(&c)

	if c.WebPush.PublicKey == "" || c.WebPush.PrivateKey == "" {
		log.Println("WARNING: VAPI keys not found, generating new ones. Please update your config")
		var err error
		c.WebPush.PrivateKey, c.WebPush.PublicKey, err = webpush.GenerateVAPIDKeys()
		if err == nil {
			newKeys, _ := json.Marshal(c.WebPush)
			log.Println("Add this key to your config:\n" + `"web_push": ` + string(newKeys))
		} else {
			log.Println("ERROR: generate vapid key pair:", err.Error())
		}
	}

	start, stop := Bootstrap(c)

	go func() {
		err := start()
		if err != nil {
			log.Println("start:", err.Error())
		}
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	_, ok := <-signals
	if ok {
		fmt.Println("terminating...")
		err := stop()
		if err != nil {
			log.Println("stop:", err.Error())
		}
	}

}
