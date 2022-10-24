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
			Base:       "https://saas.inceptiondb.io/v1",
			DatabaseID: "ab9965be-56a7-4d55-bf14-3e8b96d742c2",
			ApiKey:     "de19b3c9-ef29-445f-a0c1-92440c206246",
			ApiSecret:  "f49e48e7-a5e8-4c8a-9dec-1c0b8a7eaa5f",
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
