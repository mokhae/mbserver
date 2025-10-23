package main

import (
	"log"
	"time"

	"github.com/mokhae/mbserver"
)

func main() {
	serv := mbserver.NewServer(true, 5*time.Second)
	err := serv.ListenTCP("0.0.0.0:502")
	if err != nil {
		log.Printf("%v\n", err)
	}
	defer serv.Close()

	// Wait forever
	i := 0
	for {
		i++
		serv.HoldingRegisters[1] = uint16(i)
		time.Sleep(1 * time.Second)
	}
}
