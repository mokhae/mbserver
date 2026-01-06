package mbserver

import (
	"errors"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/goburrow/serial"
)

// ListenRTU starts the Modbus server listening to a serial device.
// For example:  err := s.ListenRTU(&serial.Config{Address: "/dev/ttyUSB0"})
func (s *Server) ListenRTU(serialConfig *serial.Config, deviceId uint8, callback PortErrorCallback) (err error) {
	port, err := serial.Open(serialConfig)
	if err != nil {
		//log.Fatalf("failed to open %s: %v\n", serialConfig.Address, err)
		log.Printf("failed to open %s: %v\n", serialConfig.Address, err)
		return err
	}
	s.ports = append(s.ports, port)

	s.portsWG.Add(1)
	go func() {
		defer s.portsWG.Done()
		//log.Printf("facceptSerialRequests %v: %v\n", port, deviceId)
		s.acceptSerialRequests(port, deviceId, callback)
	}()

	return err
}

func (s *Server) acceptSerialRequests(port serial.Port, deviceId uint8, callback PortErrorCallback) {
	//log.Printf("start acceptSerialRequests %v: %v\n", port, deviceId)
	Abuf := make([]byte, 0)
SkipFrameError:
	for {
		select {
		case <-s.portsCloseChan:

			log.Printf("Port Closed\n")
			return
		default:
		}

		buffer := make([]byte, 512)

		bytesRead, err := port.Read(buffer)
		if err != nil {
			if err == io.EOF {
				log.Printf("serial read error %v\n", err)
				break
			}
			if errors.Is(err, serial.ErrTimeout) {
				if len(Abuf) > 0 {
					if callback != nil {
						callback(fmt.Errorf("timeout error"))
					}
					Abuf = make([]byte, 0)
				}
				continue SkipFrameError
			} else {
				if callback != nil {
					callback(err)
				}
				return
			}
		}

		if bytesRead > 0 {
			//log.Printf("serial read %d bytes\n", bytesRead)
			// Set the length of the packet to the number of read bytes.
			packet := buffer[:bytesRead]
			res1 := append(Abuf, packet...)
			//if len(Abuf) > 20 {
			//	Abuf = make([]byte, 0)
			//}
			//log.Printf("serial read packet %d bytes\n", res1)
			if len(res1) > 6 {
				if res1[1] == 16 || res1[1] == 15 {
					// Write Multi Coil and register 메세지 처리 시 문제 처리
					lenByte := 7 + int(res1[6]) + 2
					if lenByte <= len(res1) {
						bRes := make([]byte, lenByte)
						copy(bRes, res1[:lenByte])
						if lenByte < len(res1) {
							Abuf = make([]byte, len(res1)-lenByte)
							copy(Abuf, res1[lenByte:])
						}
						res1 = bRes

					} else {
						Abuf = res1
						continue SkipFrameError
					}
				}

			}
			frame, err := NewRTUFrame(res1)
			if err != nil {
				if strings.Contains(err.Error(), "RTU Frame error: CRC") {
					Abuf = make([]byte, 0)
					//Abuf = res1
				} else {
					Abuf = res1
				}
				//log.Printf("bad serial frame error %v\n", err)
				//The next line prevents RTU server from exiting when it receives a bad frame. Simply discard the erroneous
				//frame and wait for next frame by jumping back to the beginning of the 'for' loop.
				//log.Printf("Keep the RTU server running!!\n")
				continue SkipFrameError
				//return
			}
			//log.Printf("New Frame : %v\n", frame)
			Abuf = make([]byte, 0)

			if frame.GetAddress() == deviceId {
				//log.Printf("slave address : %v\n", frame.GetAddress())
				request := &Request{port, frame}
				s.requestChan <- request
				<-s.responseChan

			}
			//else {
			//	log.Printf("wrong slave address")
			//}
		}
	}

}
