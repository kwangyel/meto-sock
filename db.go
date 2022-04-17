package main

import (
	"log"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

const (
	STATUS_PAID   = "PAID"
	STATUS_UNPAID = "UNPAID"
)

type SocketState struct {
	SeatId       string
	ScheduleHash string
	RemoteAddr   string
	gorm.Model
}

type queryDTO struct {
	scheduleHash string
	seatId       string
	remoteAddr   string
	id           int
}
type DB struct {
	create         chan queryDTO
	updatePaid     chan queryDTO
	restoreState   chan bool
	delete         chan queryDTO
	deleteSchedule chan queryDTO
	checkTime      chan bool
}

func newDB() *DB {
	return &DB{
		create:         make(chan queryDTO),
		updatePaid:     make(chan queryDTO),
		restoreState:   make(chan bool),
		delete:         make(chan queryDTO),
		deleteSchedule: make(chan queryDTO),
		checkTime:      make(chan bool),
	}
}

func (d *DB) run(restoreChan chan queryDTO) {
	dsn := "root:323395kt@tcp(127.0.0.1:3306)/meto_state?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Printf("[Error] Database Connection error: %v", err)
	} else {
		log.Printf("[Debug] Connected to database")
	}
	db.AutoMigrate(&SocketState{})

	for {
		select {
		case query := <-d.restoreState:
			if query {
				log.Printf("[Debug] Restoring state")
				var seats SocketState
				rows, err := db.Model(&SocketState{}).Rows()
				if err != nil {
					log.Printf("[Error] Could not read rows: %v", err)
				}

				for rows.Next() {
					db.ScanRows(rows, &seats)
					restoreChan <- queryDTO{scheduleHash: seats.ScheduleHash, seatId: seats.SeatId, remoteAddr: seats.RemoteAddr}
				}
			}

		case query := <-d.create:
			db.Create(&SocketState{ScheduleHash: query.scheduleHash, SeatId: query.seatId, RemoteAddr: query.remoteAddr})
		case query := <-d.delete:
			db.Delete(&SocketState{}, "schedule_hash = ? AND seat_id = ?", query.scheduleHash, query.seatId)
		case query := <-d.deleteSchedule:
			db.Delete(&SocketState{}, "schedule_hash = ?", query.scheduleHash)
		}

	}
}
