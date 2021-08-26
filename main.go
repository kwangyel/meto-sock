package main

import (
	"encoding/json"
	"errors"
	"flag"
	"log"
	"reflect"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/streadway/amqp"
)

var addr = flag.String("addr", ":8081", "http service address")

func main() {
	flag.Parse()
	mqChan := make(chan []byte)

	hub := newHub()
	go hub.run(mqChan)

	go runAmqp(hub, mqChan)
	// go redisSubscribe(hub)
	//

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
		_, err := strconv.Atoi(roomId)
		if err != nil {
			log.Printf("%v", err)
			log.Printf("the room id %v doesnt look like a number", roomId)
		} else if roomId != "" {
			serveWs(hub, c.Writer, c.Request, roomId)
		}
	})
	router.Run("0.0.0.0" + *addr)

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
		log.Println(err)
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
		log.Println(err)
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

	log.Println("Connected to the RabbitMQ")

	// mq := make(chan bool)
	go func() {
		for message := range messages {
			log.Printf(" > Received message: %s\n", message.Body)
			mqMsg := &MessageRequest{}
			err := json.Unmarshal([]byte(message.Body), mqMsg)
			if reflect.DeepEqual(mqMsg, MessageRequest{}) {
				err = errors.New("Cant unmarshal empty object")
				log.Println(err)
				// panic(err)
			}
			if err != nil {
				log.Println(mqMsg)
				// panic(err)
				// log.Println(err)
				// continue
			}

			switch msgtype := mqMsg.MessageType; msgtype {
			case ON_BOOK:
				log.Printf("a book message")
			case ON_LOCK_CANCEL:
				hub.broadcast <- MessageDTO{RoomId: mqMsg.RoomId, MessageType: ON_LOCK_LEAVE, SeatId: mqMsg.SeatId}
			default:
				log.Println("in the default block")

			}

			// if mqMsg.MessageType == ON_BOOK {
			// 	hub.broadcast <- MessageDTO{RoomId: mqMsg.RoomId, MessageType: ON_BOOK, BookedList: mqMsg.BookList}
			// } else {
			// 	log.Printf("not a book message")
			// }
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

// func redisSubscribe(hub *Hub) {
// 	// Redis loop here
// 	var ctx = context.Background()
// 	rdb := redis.NewClient(&redis.Options{
// 		Addr:     "localhost:6379",
// 		Password: "",
// 		DB:       0,
// 	})

// 	pong, err := rdb.Ping(ctx).Result()
// 	log.Printf(pong)
// 	if err != nil {
// 		log.Printf("ww", err)
// 		time.Sleep(3 * time.Second)
// 		err := rdb.Ping(ctx).Err()
// 		if err != nil {
// 			panic(err)
// 		}
// 	}

// 	topic := rdb.Subscribe(ctx, "ON_BOOK")
// 	rdb_channel := topic.Channel()
// 	for msg := range rdb_channel {
// 		log.Printf("there is redis msg", msg)
// 		rdbMsg := &BookingDTO{}
// 		err := json.Unmarshal([]byte(msg.Payload), rdbMsg)
// 		if err != nil {
// 			panic(err)
// 		}
// 		log.Printf("%v", rdbMsg.MessageType)
// 		if rdbMsg.MessageType == ON_BOOK {
// 			hub.broadcast <- MessageDTO{RoomId: rdbMsg.RoomId, MessageType: ON_BOOK, BookedList: rdbMsg.BookList}
// 		} else {
// 			log.Printf("not a book message")
// 		}
// 	}
// }
