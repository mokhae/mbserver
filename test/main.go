package main

import (
	"log"
	"time"

	"github.com/goburrow/serial"
	"github.com/mokhae/mbserver"
)

func main() {
	/*	serv := mbserver.NewServer(true, 5*time.Second)
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
		}*/

	serv := mbserver.NewServer(false, 5*time.Second)

	err := serv.ListenRTU(&serial.Config{
		Address:  "/dev/tty.usbserial-BG01SRBM",
		BaudRate: 19200,
		DataBits: 8,
		StopBits: 1,
		Parity:   "E",
		Timeout:  10 * time.Second}, 1, serialCallback)
	if err != nil {
		log.Printf("failed to listen, got %v", err)
	}

	serv.Debug = true
	log.Printf("Open MB Serial Server")

	for {

		log.Printf("Holding : %v", serv.HoldingRegisters[0:20])
		time.Sleep(1 * time.Second)

	}

}

func serialCallback(err error) {
	log.Printf("Serial Error : %v", err)
}
