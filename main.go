package main

import (
	"encoding/json"
	"errors"
	"flag"
	"log"
	"reflect"

	"github.com/gin-gonic/gin"
	"github.com/streadway/amqp"
)

var addr = flag.String("addr", "8081", "http service address")

func main() {
	flag.Parse()
	mqChan := make(chan []byte)

	restoreChan := make(chan queryDTO)

	//init db and update state
	dbManager := newDB()
	go dbManager.run(restoreChan)

	hub := newHub()
	go hub.run(mqChan, restoreChan, dbManager)

	go runAmqp(hub, mqChan)

	router := gin.New()

	router.LoadHTMLGlob("./*.html")
	router.GET("/room/:roomId", func(c *gin.Context) {
		c.HTML(200, "home.html", nil)
	})
	router.GET("/", func(c *gin.Context) {
		c.JSON(200, "Ok")
	})

	router.GET("/room2/:roomId", func(c *gin.Context) {
		c.HTML(200, "home2.html", nil)
	})

	router.GET("/ws/:roomId", func(c *gin.Context) {
		roomId := c.Param("roomId")
		if roomId != "" {
			serveWs(hub, c.Writer, c.Request, roomId)
		}
	})
	router.Run("127.0.0.1:" + *addr)

}

func runAmqp(hub *Hub, msg chan []byte) {
	amqpServerUrl := "amqp://guest:guest@localhost:5672/"
	connectMq, err := amqp.Dial(amqpServerUrl)
	if err != nil {
		panic(err)
	}
	defer connectMq.Close()

	channelMQ, err := connectMq.Channel()
	if err != nil {
		panic(err)
	}

	defer channelMQ.Close()
	err = channelMQ.ExchangeDeclare(
		"meto",
		"fanout",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Printf("[Error] %v", err)
	}

	q, err := channelMQ.QueueDeclare(
		"",
		false,
		false,
		true,
		false,
		nil,
	)
	if err != nil {
		log.Printf("[Error] %v", err)
	}

	err = channelMQ.QueueBind(
		q.Name,
		"",
		"meto",
		false,
		nil,
	)
	if err != nil {
		panic(err)
	}

	messages, err := channelMQ.Consume(
		q.Name,
		"",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		panic(err)
	}

	log.Printf("[Debug] Connected to the RabbitMQ")

	go func() {
		for message := range messages {
			log.Printf("[Debug] AMQP =>  %s\n", message.Body)
			mqMsg := &MessageRequest{}
			err := json.Unmarshal([]byte(message.Body), mqMsg)
			if reflect.DeepEqual(mqMsg, MessageRequest{}) {
				err = errors.New("Can't unmarshal empty object")
				log.Printf("[Error] %v", err)
			}
			if err != nil {
				log.Printf("[Error] %v", err)
			}

			switch msgtype := mqMsg.MessageType; msgtype {
			case ON_LOCK_CANCEL:
				hub.broadcast <- MessageDTO{RoomId: mqMsg.ScheduleHash, MessageType: ON_LOCK_LEAVE, SeatId: mqMsg.SeatId}
			default:
			}
		}

	}()
	for {
		select {
		case mqmsg := <-msg:
			channelMQ.Publish("meto", "", false, false, amqp.Publishing{
				DeliveryMode: 1,
				Body:         mqmsg,
			})

		}
	}
}
