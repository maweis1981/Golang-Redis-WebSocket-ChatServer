package devices

import (
	"errors"
	"fmt"
	"github.com/go-redis/redis/v7"
	"github.com/gorilla/websocket"
	"log"
	"time"
)

const (
	// used to track users that used chat. mainly for listing users in the /users api, in real world chat app
	// such devices list should be separated into devices management module.
	usersKey       = "users"
	userChannelFmt = "devices:%s:channels"
	ChannelsKey    = "channels"
)

//rdb = redis.NewClient(&redis.Options{Addr: "localhost:6379"})
var rdb = redis.NewClient(&redis.Options{
	Addr:         "localhost:6379",
	DialTimeout:  10 * time.Second,
	ReadTimeout:  30 * time.Second,
	WriteTimeout: 30 * time.Second,
	PoolSize:     2,
	PoolTimeout:  30 * time.Second,
})

type Plane struct {
	Name                string
	channelsHandler     *redis.PubSub
	WebSocketConnection *websocket.Conn

	stopListenerChan chan struct{}
	listening        bool
	MessageChan      chan redis.Message
}

//Connect connect devices to devices channels on redis
func Connect(name string, conn *websocket.Conn) (*Plane, error) {

	if _, err := rdb.SAdd(usersKey, name).Result(); err != nil {
		return nil, err
	}

	u := &Plane{
		Name:                name,
		stopListenerChan:    make(chan struct{}),
		MessageChan:         make(chan redis.Message),
		WebSocketConnection: conn,
	}

	if err := u.connect(); err != nil {
		return nil, err
	}

	return u, nil
}

func (u *Plane) Subscribe(channel string) error {

	userChannelsKey := fmt.Sprintf(userChannelFmt, u.Name)

	if rdb.SIsMember(userChannelsKey, channel).Val() {
		return nil
	}
	if err := rdb.SAdd(userChannelsKey, channel).Err(); err != nil {
		return err
	}

	return u.connect()
}

func (u *Plane) Unsubscribe(channel string) error {

	userChannelsKey := fmt.Sprintf(userChannelFmt, u.Name)

	if !rdb.SIsMember(userChannelsKey, channel).Val() {
		return nil
	}
	if err := rdb.SRem(userChannelsKey, channel).Err(); err != nil {
		return err
	}

	return u.connect()
}

func (u *Plane) connect() error {

	var c []string

	log.Println("Plane Connect Invoke.")

	c1, err := rdb.SMembers(ChannelsKey).Result()
	if err != nil {
		return err
	}
	c = append(c, c1...)

	// get all devices channels (from DB) and start subscribe
	c2, err := rdb.SMembers(fmt.Sprintf(userChannelFmt, u.Name)).Result()
	if err != nil {
		return err
	}
	c = append(c, c2...)

	if len(c) == 0 {
		log.Println("no channels to connect to for devices: ", u.Name)
		return nil
	}

	if u.channelsHandler != nil {
		if err := u.channelsHandler.Unsubscribe(); err != nil {
			return err
		}
		if err := u.channelsHandler.Close(); err != nil {
			return err
		}
	}
	if u.listening {
		u.stopListenerChan <- struct{}{}
	}

	return u.doConnect(c...)
}

func (u *Plane) doConnect(channels ...string) error {
	// subscribe all channels in one request
	pubSub := rdb.Subscribe(channels...)
	// keep channel handler to be used in unsubscribe
	u.channelsHandler = pubSub

	// The Listener
	go func() {
		u.listening = true
		log.Println("starting the listener for devices:", u.Name, "on channels:", channels)

		if err := SendCommand("general", u.Name+" Connected"); err != nil {
			log.Printf("Plane Connect Error: %s \n", err)
		}

		for {
			select {
			case msg, ok := <-pubSub.Channel():
				if !ok {
					return
				}
				u.MessageChan <- *msg

			case <-u.stopListenerChan:
				log.Println("stopping the listener for devices:", u.Name)
				return
			}
		}
	}()
	return nil
}

func (u *Plane) Disconnect() error {
	if u.channelsHandler != nil {
		if err := u.channelsHandler.Unsubscribe(); err != nil {
			return err
		}
		if err := u.channelsHandler.Close(); err != nil {
			return err
		}
	}
	if u.listening {
		u.stopListenerChan <- struct{}{}
	}

	if err := rdb.SRem(usersKey, u.Name).Err(); err != nil {
		return err
	}

	close(u.MessageChan)

	if err := SendCommand("general", u.Name+" DisConnected"); err != nil {
		log.Printf("Plane Connect Error: %s \n", err)
	}

	u.WebSocketConnection.Close()

	return nil
}

func SendCommand(channel string, content string) error {
	return rdb.Publish(channel, content).Err()
}

func List() ([]string, error) {
	return rdb.SMembers(usersKey).Result()
}

func GetChannels(username string) ([]string, error) {

	if !rdb.SIsMember(usersKey, username).Val() {
		return nil, errors.New("devices not exists")
	}

	var c []string

	c1, err := rdb.SMembers(ChannelsKey).Result()
	if err != nil {
		return nil, err
	}
	c = append(c, c1...)

	// get all devices channels (from DB) and start subscribe
	c2, err := rdb.SMembers(fmt.Sprintf(userChannelFmt, username)).Result()
	if err != nil {
		return nil, err
	}
	c = append(c, c2...)

	return c, nil
}
