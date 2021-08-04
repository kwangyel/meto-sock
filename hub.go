package main

import (
	"encoding/json"
	"log"
	"strconv"
)

type MessageRequest struct {
	RoomId      string
	MessageType string
	SeatId      string
}
type MessageDTO struct {
	RoomId      string
	MessageType string
	SeatId      string
	client      *Client
}
type MessageResponse struct {
	RoomId      string `json:"roomId"`
	MessageType string `json:"messageType"`
	// LockedList  map[string]map[*Client]bool `json:"lockedList"`
	LockedList []int           `json:"lockedList"`
	BookedList map[string]bool `json:"bookedList"`
}

type Hub struct {
	// Registered clients.
	// clients map[*Client]bool

	rooms map[string]map[*Client]bool

	// lockedList map[*Client]map[string]bool
	lockedList map[string]map[*Client]bool
	bookedList map[string]bool

	// Inbound messages from the clients.
	// broadcast chan []byte
	// broadcast chan MessageRequest
	broadcast chan MessageDTO

	// Register requests from the clients.
	register chan *Client

	// Unregister requests from clients.
	unregister chan *Client
}

func newHub() *Hub {
	return &Hub{
		broadcast:  make(chan MessageDTO),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		rooms:      make(map[string]map[*Client]bool),
		lockedList: make(map[string]map[*Client]bool),
		bookedList: make(map[string]bool),
		// clients:    make(map[*Client]bool),
	}
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			log.Printf("1 client added")
			room := h.rooms[client.roomId]
			if room == nil {
				room = make(map[*Client]bool)
				h.rooms[client.roomId] = room
			}
			room[client] = true
		case client := <-h.unregister:
			log.Printf("1 client left")
			room := h.rooms[client.roomId]

			// h.lockedList[client]
			if room != nil {
				if _, ok := room[client]; ok {
					// delete(h.lockedList, client)

					//Removing client from the locked list here
					delete(room, client)
					for k, v := range h.lockedList {
						for keys := range v {
							if keys == client {
								delete(h.lockedList, k)
							}
						}
					}
					lockedArray := make([]int, 0, len(h.lockedList))
					for key := range h.lockedList {
						i, err := strconv.Atoi(key)
						if err != nil {
							log.Printf("%v", err)
						}
						lockedArray = append(lockedArray, i)
					}

					b, err := json.Marshal(MessageResponse{RoomId: client.roomId, MessageType: ON_LOCK_LEAVE, LockedList: lockedArray, BookedList: nil})
					if err != nil {
						log.Printf("%v", err)
					}
					for cc := range room {
						select {
						// case client.send <- MessageResponse{RoomId: message.RoomId, MessageType: message.MessageType, LockedList: h.lockedList, BookedList: nil}:

						case cc.send <- b:
						default:
							log.Printf("called here")
							close(cc.send)
							delete(room, cc)
						}
					}
					close(client.send)
					if len(room) == 0 {
						delete(h.rooms, client.roomId)
					}
				}
			}
		case message := <-h.broadcast:
			room := h.rooms[message.RoomId]
			if room != nil {
				switch msgType := message.MessageType; msgType {
				case ON_LOCK:
					// h.bookedList = append(h.bookedList, message.SeatId)
					// i, err := strconv.Atoi(message.SeatId)
					// if err != nil {
					// 	log.Printf("%v", err)
					// }
					child := make(map[*Client]bool)
					child[message.client] = true
					h.lockedList[message.SeatId] = child

					lockedArray := make([]int, 0, len(h.lockedList))
					for key := range h.lockedList {
						i, err := strconv.Atoi(key)
						if err != nil {
							log.Printf("%v", err)
						}
						lockedArray = append(lockedArray, i)
					}
					log.Printf("locked list")
					log.Printf("%v", h.lockedList)
					log.Printf("locked array")
					log.Printf("%v", lockedArray)
					log.Printf("%v", h.rooms)

					log.Printf("lock")
					log.Printf("%v", h.lockedList)
					log.Printf("%v", message.SeatId)

					b, err := json.Marshal(MessageResponse{RoomId: message.RoomId, MessageType: message.MessageType, LockedList: lockedArray, BookedList: nil})
					if err != nil {
						log.Printf("%v", err)
					}

					for client := range room {
						select {
						// case client.send <- MessageResponse{RoomId: message.RoomId, MessageType: message.MessageType, LockedList: h.lockedList, BookedList: nil}:

						case client.send <- b:
						default:
							log.Printf("called here")
							close(client.send)
							delete(room, client)
						}
					}
				case ON_BOOK:
					// h.lockedList = append(h.bookedList, message.SeatId)
					// i, err := strconv.Atoi(message.SeatId)
					// if err != nil {
					// 	log.Printf("%v", err)
					// }
					h.bookedList[message.SeatId] = true

					b, err := json.Marshal(MessageResponse{RoomId: message.RoomId, MessageType: message.MessageType, LockedList: nil, BookedList: h.bookedList})

					if err != nil {
						log.Printf("%v", err)
					}
					for client := range room {
						select {
						// case client.send <- MessageResponse{RoomId: message.RoomId, MessageType: message.MessageType, LockedList: nil, BookedList: h.bookedList}:
						case client.send <- b:

						default:
							close(client.send)
							delete(room, client)
						}
					}
				case ON_LOCK_LEAVE:
					// i, err := strconv.Atoi(message.SeatId)
					// if err != nil {
					// 	log.Printf("%v", err)
					// }
					// child := make(map[*Client]bool)
					// child[message.client] = true
					// h.lockedList[message.SeatId] = child

					// h.lockedList[message.SeatId] = nil
					// delete(h.lockedList, message.client)
					b, err := json.Marshal(MessageResponse{RoomId: message.RoomId, MessageType: message.MessageType, LockedList: nil, BookedList: h.bookedList})

					if err != nil {
						log.Printf("%v", err)
					}

					for client := range room {
						select {
						// case client.send <- MessageResponse{RoomId: message.RoomId, MessageType: message.MessageType, LockedList: nil, BookedList: h.bookedList}:

						case client.send <- b:
						default:
							close(client.send)
							delete(room, client)
						}
					}

				}
			}
			if len(room) == 0 {
				delete(h.rooms, message.RoomId)
			}
		}
	}
}
