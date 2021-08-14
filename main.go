package main

import (
	"encoding/json"
	"flag"
	"log"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/streadway/amqp"
)

var addr = flag.String("addr", ":8081", "http service address")

func main() {
	flag.Parse()
	hub := newHub()
	go hub.run()
	go runAmqp(hub)
	// go redisSubscribe(hub)
	//

	router := gin.New()

	router.LoadHTMLGlob("./*.html")
	router.GET("/room/:roomId", func(c *gin.Context) {
		c.HTML(200, "home.html", nil)
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

func runAmqp(hub *Hub) {
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

	messages, err := channelMQ.Consume(
		ON_BOOK,
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
	for message := range messages {
		log.Printf(" > Received message: %s\n", message.Body)
		mqMsg := &BookingDTO{}
		err := json.Unmarshal([]byte(message.Body), mqMsg)
		if err != nil {
			panic(err)
		}

		if mqMsg.MessageType == ON_BOOK {
			hub.broadcast <- MessageDTO{RoomId: mqMsg.RoomId, MessageType: ON_BOOK, BookedList: mqMsg.BookList}
		} else {
			log.Printf("not a book message")
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
