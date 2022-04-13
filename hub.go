package main

import (
	"encoding/json"
	"log"
	"strconv"
)

type MessageRequest struct {
	ScheduleHash string
	MessageType  string
	SeatId       string
}
type MessageDTO struct {
	RoomId      string
	MessageType string
	SeatId      string
	client      *Client
}

type BookingDTO struct {
	RoomId      string `json:"roomId"`
	MessageType string `json:"messageType"`
	BookList    []int  `json:"bookList"`
}
type MessageResponse struct {
	RoomId      string   `json:"roomId"`
	MessageType string   `json:"messageType"`
	LeaveList   []string `json:"leaveList"`
	LockedList  []int    `json:"lockedList"`
}

type Hub struct {
	// Registered clients.
	// clients map[*Client]bool

	rooms map[string]map[*Client]bool

	// lockedList map[*Client]map[string]bool
	// lockedList map[string]map[*Client]bool

	// Structure is like this map[roomId]map[seatId]client
	lockedList  map[string]map[string]string
	confirmLock map[string]map[string]bool

	bookedList map[string]map[string]bool

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

		lockedList:  make(map[string]map[string]string),
		confirmLock: make(map[string]map[string]bool),
	}
}

func (h *Hub) run(mqchan chan []byte, restoreChan chan queryDTO, db *DB) {

	//init and populate lock confirmLock map
	db.restoreState <- true

	for {
		select {
		case socketState := <-restoreChan:
			//restoring confirm List first
			confirm_list := h.confirmLock[socketState.scheduleHash]
			if confirm_list == nil {
				confirm_list = make(map[string]bool)
			}
			confirm_list[socketState.seatId] = true
			h.confirmLock[socketState.scheduleHash] = confirm_list

			//restoring locked list
			locked_list := h.lockedList[socketState.scheduleHash]
			if locked_list == nil {
				locked_list = make(map[string]string)
			}
			locked_list[socketState.seatId] = socketState.remoteAddr
			h.lockedList[socketState.scheduleHash] = locked_list

			log.Printf("[Debug] Restored seat number %v of scheduleHash %v", socketState.seatId, socketState.scheduleHash)

		case client := <-h.register:
			log.Printf("%v", client.conn.UnderlyingConn().RemoteAddr().String())
			room := h.rooms[client.roomId]
			seats := h.lockedList[client.roomId]

			if room == nil {
				room = make(map[*Client]bool)
				h.rooms[client.roomId] = room
			}
			room[client] = true

			log.Printf("[Debug] ScheuldeHash %v; Count: %v;", client.roomId, len(h.rooms[client.roomId]))

			lockedArray := make([]int, 0, len(h.lockedList[client.roomId]))

			if seats != nil {
				for key := range h.lockedList[client.roomId] {
					i, err := strconv.Atoi(key)
					if err != nil {
						log.Printf("[Error] %v", err)
					}
					lockedArray = append(lockedArray, i)
				}

				b, err := json.Marshal(MessageResponse{RoomId: client.roomId, MessageType: ON_LOCK, LockedList: lockedArray})
				if err != nil {
					log.Printf("[Error] %v", err)
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

					for seatid, remoteAddress := range h.lockedList[client.roomId] {
						if confirmedSeats[seatid] {
							counter++
						} else {
							if client.conn.UnderlyingConn().RemoteAddr().String() == remoteAddress {
								log.Printf("[Debug] Seat confirmation. Removing seat %v from Locked List ", seatid)
								leaveList = append(leaveList, seatid)
								delete(h.lockedList[client.roomId], seatid)
								counter++
							}
						}
					}
					if counter > 0 {
						lockedArray := make([]int, 0, len(h.lockedList[client.roomId]))
						for seat, _ := range h.lockedList[client.roomId] {
							i, err := strconv.Atoi(seat)
							if err != nil {
								log.Printf("[Error] %v", err)
							}
							lockedArray = append(lockedArray, i)
						}

						//build json response and convert to []byte
						b, err := json.Marshal(MessageResponse{RoomId: client.roomId, LeaveList: leaveList, MessageType: ON_LOCK_LEAVE, LockedList: lockedArray})
						if err != nil {
							log.Printf("[Error] %v", err)
						}

						for clients := range room {
							select {
							// case client.send <- MessageResponse{RoomId: message.RoomId, MessageType: message.MessageType, LockedList: h.lockedList, BookedList: nil}:

							case clients.send <- b:
							default:
								close(clients.send)
								delete(room, clients)
							}
						}

					}
					close(client.send)
				}
				if len(room) == 0 {
					delete(h.rooms, client.roomId)
				}
			}

		case message := <-h.broadcast:
			room := h.rooms[message.RoomId]
			// if room != nil {
			switch msgType := message.MessageType; msgType {

			case ON_LOCK_CONFIRM:
				confirm_list := h.confirmLock[message.RoomId]
				if confirm_list == nil {
					confirm_list = make(map[string]bool)
				}
				confirm_list[message.SeatId] = true
				h.confirmLock[message.RoomId] = confirm_list

				c, err := json.Marshal(MessageRequest{ScheduleHash: message.RoomId, MessageType: ON_LOCK_CONFIRM, SeatId: message.SeatId})
				if err != nil {
					log.Printf("[Error] %v", err)
					continue
				}

				//send to amqp
				mqchan <- c

				//set in state for socket persistence
				db.create <- queryDTO{scheduleHash: message.RoomId, seatId: message.SeatId, remoteAddr: message.client.conn.UnderlyingConn().RemoteAddr().String()}

			case ON_LOCK:
				seat_client := h.lockedList[message.RoomId]
				if seat_client == nil {
					seat_client = make(map[string]string)
				}

				isSeatAvailable := true
				for key := range seat_client {
					if key == message.SeatId {
						isSeatAvailable = false
					}
				}

				if isSeatAvailable {
					seat_client[message.SeatId] = message.client.conn.UnderlyingConn().RemoteAddr().String()
					h.lockedList[message.RoomId] = seat_client

					lockedArray := make([]int, 0, len(h.lockedList[message.RoomId]))
					for key := range h.lockedList[message.RoomId] {
						i, err := strconv.Atoi(key)
						if err != nil {
							log.Printf("[Error] %v", err)
						}
						lockedArray = append(lockedArray, i)
					}

					b, err := json.Marshal(MessageResponse{RoomId: message.RoomId, MessageType: message.MessageType, LockedList: lockedArray})
					if err != nil {
						log.Printf("[Error] %v", err)
					}

					for client := range room {
						if client == message.client {
							a, err := json.Marshal(MessageResponse{RoomId: message.RoomId, MessageType: ON_LOCK_ACK})
							if err != nil {
								log.Printf("[Error] %v", err)
							}
							select {
							case message.client.send <- a:
							default:
								close(message.client.send)
								delete(room, message.client)
							}
						} else {
							select {
							case client.send <- b:
							default:
								close(client.send)
								delete(room, client)
							}
						}
					}
				} else {
					a, err := json.Marshal(MessageResponse{RoomId: message.RoomId, MessageType: ON_LOCK_FAIL})
					if err != nil {
						log.Printf("[Error] %v", err)
					}
					select {
					case message.client.send <- a:
					default:
						close(message.client.send)
						delete(room, message.client)
					}
				}
			case ON_LOCK_LEAVE:
				//remove from Locked List
				delete(h.lockedList[message.RoomId], message.SeatId)

				//remove from Confirm List
				lockConfirm := h.confirmLock[message.RoomId]
				if lockConfirm[message.SeatId] {
					delete(lockConfirm, message.SeatId)
					h.confirmLock[message.RoomId] = lockConfirm
				}

				leaveList := make([]string, 0)
				leaveList = append(leaveList, message.SeatId)

				b, err := json.Marshal(MessageResponse{RoomId: message.RoomId, LeaveList: leaveList, MessageType: message.MessageType, LockedList: nil})
				if err != nil {
					log.Printf("[Error] %v", err)
				}

				db.delete <- queryDTO{scheduleHash: message.RoomId, seatId: message.SeatId}
				log.Println("ON cancel received")
				for client := range room {
					select {
					case client.send <- b:
					default:
						close(client.send)
						delete(room, client)
					}
				}
				//remove from socket state in db

			}
			// }
			// if len(room) == 0 {
			// 	delete(h.rooms, message.RoomId)
			// }
		}
	}
}
