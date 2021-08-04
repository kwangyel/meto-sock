package main

import (
	"log"
	"strconv"
)

type MessageRequest struct {
	RoomId      string `json:"roomId"`
	MessageType string `json:"messageType"`
	SeatId      string `json:"seatId"`
}
type MessageResponse struct {
	RoomId      string       `json:"roomId"`
	MessageType string       `json:"messageType"`
	LockedList  map[int]bool `json:"lockedList"`
	BookedList  map[int]bool `json:"bookedList"`
}

type Hub struct {
	// Registered clients.
	// clients map[*Client]bool

	rooms map[string]map[*Client]bool

	lockedList map[int]bool
	bookedList map[int]bool

	// Inbound messages from the clients.
	// broadcast chan []byte
	broadcast chan MessageRequest

	// Register requests from the clients.
	register chan *Client

	// Unregister requests from clients.
	unregister chan *Client
}

func newHub() *Hub {
	return &Hub{
		broadcast:  make(chan MessageRequest),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		rooms:      make(map[string]map[*Client]bool),
		lockedList: make(map[int]bool),
		bookedList: make(map[int]bool),
		// clients:    make(map[*Client]bool),
	}
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			room := h.rooms[client.roomId]
			if room == nil {
				room = make(map[*Client]bool)
				h.rooms[client.roomId] = room
			}
			room[client] = true
		case client := <-h.unregister:
			room := h.rooms[client.roomId]
			if room != nil {
				if _, ok := room[client]; ok {
					delete(room, client)
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
					i, err := strconv.Atoi(message.SeatId)
					if err != nil {
						log.Printf("%v", err)
					}
					h.lockedList[i] = true
					log.Printf("lock")
					log.Printf("%v", h.lockedList)
					log.Printf("%v", message.SeatId)

					for client := range room {
						select {
						case client.send <- MessageResponse{RoomId: message.RoomId, MessageType: message.MessageType, LockedList: h.lockedList, BookedList: nil}:

						default:
							close(client.send)
							delete(room, client)
						}
					}
				case ON_BOOK:
					// h.lockedList = append(h.bookedList, message.SeatId)
					i, err := strconv.Atoi(message.SeatId)
					if err != nil {
						log.Printf("%v", err)
					}
					h.bookedList[i] = true
					for client := range room {
						select {
						case client.send <- MessageResponse{RoomId: message.RoomId, MessageType: message.MessageType, LockedList: nil, BookedList: h.bookedList}:

						default:
							close(client.send)
							delete(room, client)
						}
					}
				case ON_LOCK_LEAVE:
					i, err := strconv.Atoi(message.SeatId)
					if err != nil {
						log.Printf("%v", err)
					}
					h.lockedList[i] = false
					for client := range room {
						select {
						case client.send <- MessageResponse{RoomId: message.RoomId, MessageType: message.MessageType, LockedList: nil, BookedList: h.bookedList}:

						default:
							close(client.send)
							delete(room, client)
						}
					}

				}
				for client := range room {
					select {
					case client.send <- MessageResponse{RoomId: message.RoomId, MessageType: message.MessageType, LockedList: h.lockedList, BookedList: h.bookedList}:

					default:
						close(client.send)
						delete(room, client)
					}
				}
				if len(room) == 0 {
					delete(h.rooms, message.RoomId)
				}

			}
		}
	}
}
