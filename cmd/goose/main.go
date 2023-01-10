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
			DatabaseID: "1b5b8fef-db19-4308-852d-d0c61eda7143",
			ApiKey:     "973417a6-ea20-4ac9-8b9f-6f3db9213a01",
			ApiSecret:  "ebac1daf-48fc-4e88-a6ba-04ebf1b48125",
		},
	}
	goconfig.Read(&c)

	start, stop := Bootstrap(c)

	go func() {
		err := start()
		if err != nil {
			log.Println("start:", err.Error())
		}
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT)
	_, ok := <-signals
	if ok {
		fmt.Println("terminating...")
		err := stop()
		if err != nil {
			log.Println("stop:", err.Error())
		}
	}

}
