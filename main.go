package main

import (
	"flag"

	"github.com/gin-gonic/gin"
)

var addr = flag.String("addr", ":8080", "http service address")

// func getParam(r *http.Request) string {
// 	p := strings.Split(r.URL.Path, "/")
// 	log.Println(p)
// 	log.Println("this")
// 	if len(p) == 3 {
// 		return p[2]
// 	} else {
// 		return ""
// 	}
// }

// func serveHome(w http.ResponseWriter, r *http.Request) {
// 	log.Println(r.URL)

// 	if r.URL.Path != "/" {
// 		http.Error(w, "Not found", http.StatusNotFound)
// 		return
// 	}
// 	if r.Method != "GET" {
// 		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
// 		return
// 	}
// 	// roomId := strings.Split(r.URL.Path, "/")
// 	// fmt.Printf(r.URL.Path)
// 	http.ServeFile(w, r, "home.html")
// }

func main() {
	flag.Parse()
	hub := newHub()
	go hub.run()

	router := gin.New()
	router.LoadHTMLFiles("home.html")
	router.GET("/room/:roomId", func(c *gin.Context) {
		c.HTML(200, "home.html", nil)
	})

	router.GET("/ws/:roomId", func(c *gin.Context) {
		roomId := c.Param("roomId")
		serveWs(hub, c.Writer, c.Request, roomId)
	})
	router.Run("0.0.0.0" + *addr)

	// http.HandleFunc("/", serveHome)
	// http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
	// 	// roomId := getParam(r)
	// 	// log.Println("this is the log")
	// 	// log.Println(roomId)
	// 	// if roomId != "" {
	// 	// 	serveWs(hub, w, r, roomId)
	// 	// }
	// 	log.Println("this is the handler for ws")
	// 	log.Println(getParam(r))
	// 	serveWs(hub, w, r, "1")
	// 	// serveWs(hub, w, r, "1")
	// })
	// err := http.ListenAndServe(*addr, nil)
	// if err != nil {
	// 	log.Fatal("ListenAndServe: ", err)
	// }
}
