// Copyright (c) 2023 Janik Liebrecht
// Use of this source code is governed by the MIT License that can be found in the LICENSE file.

package artemis

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/go-stomp/stomp/v3"
	"github.com/rs/zerolog/log"
)

// unlimited is used to receive an unlimited number of messages.
const unlimited = 0

// A Receiver receives messages of type T from the artemis broker.
type Receiver[T any] struct {

	// Addr is the address of the broker.
	Addr string

	User string

	Pass string

	// Dest is the default destination.
	Dest string

	// PubSub configures the type of destination.
	// 'true' for the publish-subscribe pattern (topics),
	// 'false' for the producer-consumer pattern (queues).
	PubSub bool

	// Enc specifies the encoding.
	// The default encoding is gob.
	Enc encoding

	conn *stomp.Conn

	IsConnecting bool

	RequestReconnect bool

	SystemExitCommand bool

	sync.Mutex
}

func (s *Receiver[T]) Init() error {
	err := s.InitConn()
	if err == nil {
		go s.ReconnectingWorker()
	}
	return err
}

func (r *Receiver[T]) InitConn() error {
	conn, err := stomp.Dial("tcp", r.Addr, stomp.ConnOpt.Login(r.User, r.Pass))
	if err != nil {
		return err
	}

	r.conn = conn

	return err

}

func (s *Receiver[T]) ReconnectingWorker() {
	for {
		var req_connect bool
		s.Lock()
		req_connect = s.RequestReconnect
		s.Unlock()

		if req_connect {
			s.Lock()
			s.IsConnecting = true
			s.Unlock()

			for {
				log.Info().Msg("artemis receiver : try to reconnecting")
				err := s.InitConn()
				if err == nil {
					log.Info().Msg("artemis receiver : connected")
					time.Sleep(2 * time.Second)
					s.Lock()
					s.IsConnecting = false
					s.RequestReconnect = false
					s.Unlock()
					break
				}

				log.Info().Msgf("artemis receiver : failed reconnecting try again in 10 secs %#v", err)
				time.Sleep(10 * time.Second)
			}
		}

		time.Sleep(10 * time.Second)
	}
}

func (r *Receiver[T]) Disconnect(system_exit bool) error {

	if system_exit {
		r.Lock()
		r.SystemExitCommand = true
		r.Unlock()
	}

	return r.conn.Disconnect()
}

// ReceiveMessages receives a specified number of messages from a specified destination.
// If number is set to 0, it will receive an unlimited number of messages.
func (r *Receiver[T]) ReceiveMessages(destination string, number uint64, handler func(msg *T)) error {

	interval_reconnect := 10 * time.Second

	for {

	Connecting:
		sub, err := r.conn.Subscribe(destination, stomp.AckAuto,
			stomp.SubscribeOpt.Header("subscription-type", r.subType()))
		if err != nil {
			log.Error().Msgf("could not subscribe to queue %s: %v, wait to reconnect", destination, err)

			r.Lock()
			if !r.IsConnecting {
				r.RequestReconnect = true
			}
			r.Unlock()

			time.Sleep(interval_reconnect)
			goto Connecting
		}

		var i uint64 = 0
		for ; number == unlimited || i < number; i++ {
			msg := <-sub.C
			if msg.Err != nil {
				log.Error().Msgf("received an error: %v , wait to reconnect", msg.Err)
				break
			}
			m, err := decode[T](msg.Body, r.Enc)
			if err != nil {
				log.Error().Msgf("failed to decode message: %v: %v", msg.Header, err)
				continue
			}
			handler(m)
		}

		r.Lock()
		exit := r.SystemExitCommand
		r.Unlock()

		if exit || i == number {
			break
		}

		r.Lock()
		if !r.IsConnecting {
			r.RequestReconnect = true
		}
		r.Unlock()

		time.Sleep(interval_reconnect)
	}
	return nil
}

// ReceiveFrom receives messages from a specified destination.
func (r *Receiver[T]) ReceiveFrom(destination string, handler func(msg *T)) error {
	return r.ReceiveMessages(destination, unlimited, handler)
}

// Receive receives messages from the default destination.
func (r *Receiver[T]) Receive(handler func(msg *T)) error {
	if r.Dest == "" {
		return fmt.Errorf("no default destination specified")
	}
	return r.ReceiveFrom(r.Dest, handler)
}

func (r *Receiver[T]) subType() string {
	if r.PubSub {
		return "MULTICAST"
	} else {
		return "ANYCAST"
	}
}

func decode[T any](message []byte, enc encoding) (*T, error) {
	switch enc {
	case EncodingGob:
		return decodeGob[T](message)
	case EncodingJson:
		return decodeJson[T](message)
	default:
		return nil, fmt.Errorf("unknown encoding: %v", enc)
	}
}

func decodeGob[T any](message []byte) (*T, error) {
	gob.Register(*new(T))
	buff := bytes.NewBuffer(message)
	dec := gob.NewDecoder(buff)
	var msg any
	err := dec.Decode(&msg)
	if err != nil {
		return nil, fmt.Errorf("could not decode gob: %v", err)
	}
	m := msg.(T)
	return &m, nil
}

func decodeJson[T any](message []byte) (*T, error) {
	var msg T
	err := json.Unmarshal(message, &msg)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal json: %v", err)
	}
	return &msg, nil
}
