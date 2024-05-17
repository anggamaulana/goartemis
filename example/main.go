package main

import (
	"fmt"
	"time"

	"github.com/anggamaulana/goartemis/artemis"
)

const (
	brokerAddr  = "10.0.8.116:61616"
	destination = "coster_test_queue"
	user        = "admin"
	password    = "dordordor"
)

func main() {

	// create a receiver that receives messages of type string
	receiver := &artemis.Receiver[map[string]interface{}]{
		Addr:   brokerAddr,
		Dest:   destination,
		User:   user,
		Pass:   password,
		PubSub: false,
		Enc:    1,
	}

	err := receiver.Init()
	if err != nil {
		panic(err)
	}
	fmt.Println("receiver connected")

	// create a message handler
	handler := func(msg *map[string]interface{}) {
		fmt.Println("receive", msg)
	}

	// start receiving messages in a separate goroutine
	go func() {
		err := receiver.ReceiveFrom(destination, handler)
		if err != nil {
			fmt.Println(err)
		}
	}()

	// create a sender and send two messages
	sender := &artemis.Publisher{
		Addr:   brokerAddr,
		Dest:   destination,
		User:   user,
		Pass:   password,
		PubSub: false,
		Enc:    1,
	}

	err = sender.Init()

	if err != nil {
		panic(err)
	}
	fmt.Println("publisher connected")

	err = sender.Send(map[string]interface{}{"status": "ok1"}, map[string]interface{}{"status": "ok2"})
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println("send successfully")

	time.Sleep(10 * time.Second)

	fmt.Println("scenario disconnect")
	sender.Disconnect()
	receiver.Disconnect(false)

	for i := 0; i < 10; i++ {
		err = sender.Send(map[string]interface{}{"status": "ok1"}, map[string]interface{}{"status": "ok2"})
		if err != nil {
			fmt.Println(err)
		}
		time.Sleep(5 * time.Second)
	}

}
