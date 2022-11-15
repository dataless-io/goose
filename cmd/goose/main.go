package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/fulldump/goconfig"

	"goose/inceptiondb"
)

type Config struct {
	Addr      string `json:"addr"`
	Statics   string `json:"statics"`
	Inception inceptiondb.Config
}

func main() {

	c := Config{
		Addr: ":8080", // default address
		Inception: inceptiondb.Config{
			Base: "https://saas.inceptiondb.io/v1",

			// Development credentials by default:
			DatabaseID: "c3b4b9ea-1f16-4b10-826a-d3190234a440",
			ApiKey:     "948b9e09-b538-44ac-9310-61484a5c4782",
			ApiSecret:  "72860aba-f318-4737-9849-b204ae796c4f",
		},
	}
	goconfig.Read(&c)

	start, stop := Bootstrap(c)

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT)
	go func() {
		_, ok := <-signals
		if ok {
			fmt.Println("terminating...")
			err := stop()
			if err != nil {
				log.Println("stop:", err.Error())
			}
		}
	}()

	err := start()
	if err != nil {
		log.Println("start:", err.Error())
	}
}
