package main

import (
	"log"
	"net/http"

	restapi "github.com/Clint-Mathews/chaosbox/apps/store/rest-api"
)

func main() {
	log.Println("server listening on :8080")
	if err := http.ListenAndServe(":8080", restapi.NewHandler()); err != nil {
		log.Fatal(err)
	}
}
