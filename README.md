Go library to send and receive messages to/from ActiveMQ Artemis using github.com/go-stomp/stomp/v3

inspired by https://github.com/JanikL/go-artemis, I add some functionality for my need :
- initialize connection once and reuse it in all project
- add user and password
- simple approach to reconnecting when publisher throwing error or consumer not receiving any message 

example scenario when disconecting from server 

```go
package main

import (
	"fmt"
	"time"

	"github.com/anggamaulana/goartemis/artemis"
)

const (
	brokerAddr  = "127.0.0.1:61616"
	destination = "artemis_test_queue"
	user        = "admin"
	password    = "admin"
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


```
