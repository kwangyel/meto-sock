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
	LeaveList  []string        `json:"leaveList"`
	LockedList []int           `json:"lockedList"`
	BookedList map[string]bool `json:"bookedList"`
}

type Hub struct {
	// Registered clients.
	// clients map[*Client]bool

	rooms map[string]map[*Client]bool

	// lockedList map[*Client]map[string]bool
	// lockedList map[string]map[*Client]bool

	// Structure is like this map[roomId]map[seatId]client
	lockedList  map[string]map[string]*Client
	confirmLock map[string]map[string]*Client

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

		lockedList:  make(map[string]map[string]*Client),
		confirmLock: make(map[string]map[string]*Client),
		bookedList:  make(map[string]bool),
		// clients:    make(map[*Client]bool),
	}
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			room := h.rooms[client.roomId]
			seats := h.lockedList[client.roomId]
			if room == nil {
				room = make(map[*Client]bool)
				h.rooms[client.roomId] = room
				// TODO: might also need to check if the roomId(aka scheduleId) actually exists
			}
			room[client] = true

			if seats != nil {
				lockedArray := make([]int, 0, len(h.lockedList[client.roomId]))
				for key := range h.lockedList[client.roomId] {
					i, err := strconv.Atoi(key)
					if err != nil {
						log.Printf("%v", err)
					}
					lockedArray = append(lockedArray, i)
				}

				b, err := json.Marshal(MessageResponse{RoomId: client.roomId, MessageType: ON_LOCK, LockedList: lockedArray, BookedList: nil})
				if err != nil {
					log.Printf("%v", err)
				}
				select {
				case client.send <- b:
				default:
					close(client.send)
					delete(room, client)
				}
			}

		case client := <-h.unregister:
			room := h.rooms[client.roomId]
			counter := 0

			if room != nil {
				if _, ok := room[client]; ok {
					delete(room, client)

					//Removing client and associated seat from the locked list on disconnect
					// if they have confirmed the seat the seat should be in the conrim locks and not removed
					confirmedSeats := h.confirmLock[client.roomId]
					leaveList := make([]string, 0)

					for k, v := range h.lockedList[client.roomId] {
						log.Printf(k)
						log.Printf("%v", h.lockedList)
						log.Printf("%v", confirmedSeats)
						if confirmedSeats[k] != nil {
							counter++
						} else {
							if v == client {
								leaveList = append(leaveList, k)
								delete(h.lockedList[client.roomId], k)
								counter++
							}
						}
					}
					if counter > 0 {
						lockedArray := make([]int, 0, len(h.lockedList[client.roomId]))
						for key := range h.lockedList[client.roomId] {
							i, err := strconv.Atoi(key)
							if err != nil {
								log.Printf("%v", err)
							}
							lockedArray = append(lockedArray, i)
						}

						//build json response and convert to []byte
						b, err := json.Marshal(MessageResponse{RoomId: client.roomId, LeaveList: leaveList, MessageType: ON_LOCK_LEAVE, LockedList: lockedArray, BookedList: nil})
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

				case ON_LOCK_CONFIRM:
					seat_client := h.confirmLock[message.RoomId]
					if seat_client == nil {
						seat_client = make(map[string]*Client)
					}
					seat_client[message.SeatId] = message.client
					h.confirmLock[message.RoomId] = seat_client

					b, err := json.Marshal(MessageResponse{RoomId: message.RoomId, MessageType: ON_LOCK_CONFIRM, LockedList: nil, BookedList: nil})
					if err != nil {
						log.Printf("%v", err)
					}
					select {
					case message.client.send <- b:
					default:
						close(message.client.send)
						delete(room, message.client)
					}
				case ON_LOCK:
					seat_client := h.lockedList[message.RoomId]
					if seat_client == nil {
						seat_client = make(map[string]*Client)
					}
					seat_client[message.SeatId] = message.client
					h.lockedList[message.RoomId] = seat_client
					log.Printf("%v", h.lockedList)

					lockedArray := make([]int, 0, len(h.lockedList[message.RoomId]))
					for key := range h.lockedList[message.RoomId] {
						i, err := strconv.Atoi(key)
						if err != nil {
							log.Printf("%v", err)
						}
						lockedArray = append(lockedArray, i)
					}

					b, err := json.Marshal(MessageResponse{RoomId: message.RoomId, MessageType: message.MessageType, LockedList: lockedArray, BookedList: nil})
					if err != nil {
						log.Printf("%v", err)
					}

					for client := range room {
						select {
						case client.send <- b:
						default:
							close(client.send)
							delete(room, client)
						}
					}
				case ON_BOOK:
					// TODO: integrate redis here
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
					delete(h.lockedList[message.RoomId], message.SeatId)

					lockConfirm := h.confirmLock[message.RoomId]
					if lockConfirm[message.SeatId] != nil {
						delete(lockConfirm, message.SeatId)
						h.confirmLock[message.RoomId] = lockConfirm
					}

					// lockedArray := make([]int, 0, len(h.lockedList[message.RoomId]))
					// for key := range h.lockedList[message.RoomId] {
					// 	i, err := strconv.Atoi(key)
					// 	if err != nil {
					// 		log.Printf("%v", err)
					// 	}
					// 	lockedArray = append(lockedArray, i)
					// }
					leaveList := make([]string, 0)
					leaveList = append(leaveList, message.SeatId)

					b, err := json.Marshal(MessageResponse{RoomId: message.RoomId, LeaveList: leaveList, MessageType: message.MessageType, LockedList: nil, BookedList: nil})
					if err != nil {
						log.Printf("%v", err)
					}
					log.Printf("%v", leaveList)

					for client := range room {
						select {
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
