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

type encoding int

const (
	EncodingGob encoding = iota
	EncodingJson
)

// A Publisher sends messages to the artemis broker.
type Publisher struct {

	// Addr is the address of the broker.
	Addr string

	// Dest is the default destination.
	Dest string

	User string

	Pass string

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

	sync.Mutex
}

func (s *Publisher) Init() error {
	err := s.InitConn()
	if err == nil {
		go s.ReconnectingWorker()
	}
	return err
}

func (s *Publisher) InitConn() error {
	conn, err := stomp.Dial("tcp", s.Addr, stomp.ConnOpt.Login(s.User, s.Pass))
	if err != nil {
		return err
	}

	s.conn = conn

	return err

}

func (s *Publisher) ReconnectingWorker() {
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
				log.Info().Msg("artemis publisher : try to reconnecting")
				err := s.InitConn()
				if err == nil {
					log.Info().Msg("artemis publisher : connected")
					time.Sleep(2 * time.Second)
					s.Lock()
					s.IsConnecting = false
					s.RequestReconnect = false
					s.Unlock()
					break
				}

				log.Info().Msgf("artemis publisher : failed reconnecting try again in 10 secs %#v", err)
				time.Sleep(10 * time.Second)
			}
		}

		time.Sleep(10 * time.Second)
	}
}

func (s *Publisher) Disconnect() error {

	return s.conn.Disconnect()
}

// SendTo sends messages to a specified destination.
func (s *Publisher) SendTo(destination string, messages ...any) error {

	destType := s.destType()
	for _, msg := range messages {
		m, err := encode(msg, s.Enc)
		if err != nil {
			return fmt.Errorf("failed to encode message: %v: %v", msg, err)
		}
		err = s.conn.Send(destination, "application/json", m,
			stomp.SendOpt.Header("destination-type", destType))
		if err != nil {

			s.Lock()
			if !s.IsConnecting {
				s.RequestReconnect = true
			}
			s.Unlock()

			return fmt.Errorf("could not send to destination %s: %v", destination, err)
		}
	}
	return nil
}

// Send sends messages to the default destination.
func (s *Publisher) Send(messages ...any) error {
	if s.Dest == "" {
		return fmt.Errorf("no default destination specified")
	}
	return s.SendTo(s.Dest, messages...)
}

func (s *Publisher) destType() string {
	if s.PubSub {
		return "MULTICAST"
	} else {
		return "ANYCAST"
	}
}

func encode(message any, enc encoding) ([]byte, error) {
	switch enc {
	case EncodingGob:
		return encodeGob(message)
	case EncodingJson:
		return encodeJson(message)
	default:
		return nil, fmt.Errorf("unknown encoding: %v", enc)
	}
}

func encodeGob(message any) ([]byte, error) {
	gob.Register(message)
	buff := bytes.Buffer{}
	enc := gob.NewEncoder(&buff)
	err := enc.Encode(&message) // Pass pointer to interface so Encode sees a value of interface type.
	if err != nil {
		return nil, fmt.Errorf("could not encode as gob: %v", err)
	}
	return buff.Bytes(), nil
}

func encodeJson(message any) ([]byte, error) {
	b, err := json.Marshal(message)
	if err != nil {
		return nil, fmt.Errorf("could not marshal as json: %v", err)
	}
	return b, nil
}
