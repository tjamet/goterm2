package iterm2

import (
	"fmt"
	"sync"

	proto "github.com/gogo/protobuf/proto"
	"github.com/gorilla/websocket"
	"github.com/tjamet/goterm2/api"
)

//go:generate go run generate/main.go

type Logger interface {
	Debugf(format string, args ...interface{})
	Infof(format string, args ...interface{})
	Warnf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
	Fatalf(format string, args ...interface{})
}

type NopLogger struct{}

func (NopLogger) Debugf(format string, args ...interface{}) {}
func (NopLogger) Infof(format string, args ...interface{})  {}
func (NopLogger) Warnf(format string, args ...interface{})  {}
func (NopLogger) Errorf(format string, args ...interface{}) {}
func (NopLogger) Fatalf(format string, args ...interface{}) {
	panic(fmt.Sprintf(format, args...))
}

// MessageIDError is the error returned when there is already a message with the same ID going on
// Or when receiving an unknown message ID
type MessageIDError struct {
	Message string
	ID      int64
}

func (e MessageIDError) Error() string {
	return fmt.Sprintf("%s. message ID: %d", e.Message, e.ID)
}

// ITerm2 holds the context for a new ITerm client
type ITerm2 struct {
	notifier       Notifier
	conn           *websocket.Conn
	ids            <-chan int64
	inFlyResponses map[int64]chan *api.ServerOriginatedMessage
	access         *sync.Mutex
	logger         Logger
}

func newSequentialIDs() <-chan int64 {
	c := make(chan int64)
	go func() {
		i := int64(1)
		for {
			c <- i
			i++
		}
	}()
	return c
}

// New instanciates a new ITerm extension client
func New() (*ITerm2, error) {
	c, err := NewConnection()
	iterm := &ITerm2{
		conn:           c,
		ids:            newSequentialIDs(),
		inFlyResponses: map[int64]chan *api.ServerOriginatedMessage{},
		access:         &sync.Mutex{},
		logger:         NopLogger{},
	}
	go iterm.DispatchMessages()
	return iterm, err
}

// Logger changes the default nopLogger for a custom one
func (I *ITerm2) Logger(l Logger) {
	I.logger = l
	I.logger.Debugf("Setting up the logger: %p", l)
}

// NewID generates a new unique sequential request ID
func (I *ITerm2) NewID() *int64 {
	i := <-I.ids
	return &i
}

func (I *ITerm2) storeResponseChannel(ID int64, c chan *api.ServerOriginatedMessage) error {
	I.access.Lock()
	defer I.access.Unlock()
	if old, ok := I.inFlyResponses[ID]; ok {
		I.logger.Errorf("Response channel already exist for message ID %d (old chan %p, new chan %p)", ID, old, c)
		return MessageIDError{"duplicated in-fly message", ID}
	}
	I.inFlyResponses[ID] = c
	I.logger.Debugf("stored in-flight message ID %d response channel", ID)
	return nil
}

func (I *ITerm2) getResponseChannel(ID int64) (chan *api.ServerOriginatedMessage, error) {
	I.access.Lock()
	defer I.access.Unlock()
	if c, ok := I.inFlyResponses[ID]; ok {
		I.logger.Debugf("found in-flight message ID %d", ID)
		return c, nil
	}
	I.logger.Warnf("unkown message ID %d", ID)
	return nil, MessageIDError{"unknown response channel", ID}
}

func (I *ITerm2) forgetMessageID(ID int64) error {
	I.access.Lock()
	defer I.access.Unlock()
	if _, ok := I.inFlyResponses[ID]; ok {
		delete(I.inFlyResponses, ID)
		I.logger.Debugf("forgetting about in-flight message ID %d", ID)
		return nil
	}
	I.logger.Warnf("unkown message ID %d to forget about, nothing has been done", ID)
	return MessageIDError{"unknown response channel", ID}
}

func (I *ITerm2) SendMessage(m *api.ClientOriginatedMessage) (cout <-chan *api.ServerOriginatedMessage, err error) {
	c := make(chan *api.ServerOriginatedMessage)
	cout = c
	defer func() {
		if err != nil {
			I.logger.Errorf("failed to send message ID %d: %s. Removing it from possible responses", m.GetId(), err)
			I.forgetMessageID(m.GetId())
			if c != nil {
				close(c)
			}
		} else {
			I.logger.Infof("Sent message ID %d", m.GetId())
		}
	}()
	s, err := proto.Marshal(m)
	if err != nil {
		return
	}
	err = I.storeResponseChannel(m.GetId(), c)
	if err != nil {
		return
	}
	err = I.conn.WriteMessage(websocket.BinaryMessage, s)
	return
}

func (I *ITerm2) DispatchMessages() {
	if I.conn != nil {
		I.logger.Debugf("starting receive message loop")
		for {
			t, mb, err := I.conn.ReadMessage()
			if err != nil {
				I.logger.Errorf("failed to read message from websocket: %s", err)
				return
			}
			go I.dispatchOneMessage(t, mb)
		}
	} else {
		I.logger.Errorf("no connection available to read messages from, aborting")
	}
}

func (I *ITerm2) dispatchOneMessage(t int, mb []byte) {
	if t == websocket.BinaryMessage {
		m := api.ServerOriginatedMessage{}
		err := proto.Unmarshal(mb, &m)
		if err != nil {
			I.logger.Errorf("failed to de-serialize message: %s, ignoting it", err)
			return
		}
		if m.GetNotification() != nil {
			I.dispatchNotification(m.GetNotification())
			return
		}
		c, err := I.getResponseChannel(m.GetId())
		if err != nil {
			I.logger.Errorf("failed to get response chanel for message ID %d: %s", m.GetId(), err)
			return
		}
		c <- &m
		I.forgetMessageID(m.GetId())
		close(c)
		I.logger.Infof("done processing message ID %d", m.GetId())
	} else {
		I.logger.Warnf("received unexpected message type %d, ignoring it", t)
	}
}
