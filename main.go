package main

import (
	"bytes"
	"flag"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"time"
)

type Client struct {
	hub     *Hub
	connect *websocket.Conn
	send    chan []byte
}

type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.clients[client] = true
		case client := <-h.register:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
		case message := <-h.broadcast:
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
		}
	}
}


func newHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

func main() {
	var address = flag.String("address", ":5000", "http service address")
	hub := newHub()
	go hub.run()

	http.HandleFunc("/", home)
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWS(hub, w, r)
	})
	if err := http.ListenAndServe(*address, nil); err != nil {
		log.Fatalf("Error of listen server: %s", err)
	}

}

func home(w http.ResponseWriter, r *http.Request) {
	log.Println(r.URL)
	if r.URL.Path != "/" {
		http.Error(w, "Not found", http.StatusNotFound)
	}
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
	http.ServeFile(w, r, "home.html")
}

func (c *Client) readMsg() {
	defer func(){
		c.hub.register <- c
		if err := c.connect.Close(); err != nil {
			log.Println(err)
		}
	}()
	c.connect.SetReadLimit(512)
	if err := c.connect.SetReadDeadline(time.Now().Add(time.Minute)); err != nil {
		log.Println(err)
	}
	c.connect.SetPongHandler(func(string) error {
		c.connect.SetReadDeadline(time.Now().Add(time.Minute))
		return nil
	})
	for {
		_, message, err := c.connect.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Println(err)
			}
			break
		}
		message = bytes.TrimSpace(bytes.Replace(message, []byte{'\n'}, []byte{' '}, -1))
		c.hub.broadcast <- message
	}
}

func (c *Client) writeMsg() {
	ticker := time.NewTicker(50 * time.Second)
	defer func() {
		ticker.Stop()
		if err := c.connect.Close(); err != nil {
			log.Println(err)
		}
	}()
	for {
		select {
		case message, ok := <-c.send:
			c.connect.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.connect.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			w, err := c.connect.NextWriter(websocket.TextMessage)
			if err != nil {
				log.Println(err)
				return
			}
			if _, err := w.Write(message); err != nil {
				log.Println(err)
			}
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}
		case <- ticker.C:
			c.connect.SetWriteDeadline(time.Now().Add(10* time.Second))
			if err := c.connect.WriteMessage(websocket.PingMessage, nil); err != nil{
				log.Println(err)
				return
			}
		}
	}
}

func serveWS(hub *Hub, w http.ResponseWriter, r *http.Request) {
	var upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
	}

	client := &Client{
		hub:     hub,
		connect: conn,
		send:    make(chan []byte, 256),
	}

	go client.writeMsg()
	go client.readMsg()
}
