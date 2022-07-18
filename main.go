package main

import (
	"gin-limited/route"
	"log"
)

func main() {
	r := route.NewRouter()
	err := r.Run(":8080")
	if err != nil {
		log.Fatalln(err)
	}
}
