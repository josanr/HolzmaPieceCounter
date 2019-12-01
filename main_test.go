package main

import (
	"log"
	"net/http"
	"testing"

	"github.com/josanr/HolzmaPieceCounter/monitor"
	"github.com/spf13/viper"
)

func Test_requestor(t *testing.T) {
	theAPIClient = http.DefaultClient
	viper.SetConfigName("config")
	viper.AddConfigPath(".")
	err := viper.ReadInConfig()
	if err != nil {
		log.Fatal("Fatal error reading config file")
	}

	err = markBoard(monitor.Board{})
	if err != nil {
		log.Fatal(err)
	}
}
