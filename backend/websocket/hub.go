package websocket

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type WSMessage struct {
	Type       string      `json:"type"`
	Data       interface{} `json:"data"`
	Timestamp  int64       `json:"timestamp"`
	FromDevice string      `json:"from_device,omitempty"`
}

type ClipboardSyncData struct {
	ID           uint   `json:"id"`
	Content      string `json:"content"`
	Translation  string `json:"translation,omitempty"`
	DeviceName   string `json:"device_name"`
	IsTranslated bool   `json:"is_translated"`
	ContentType  string `json:"content_type"`
	CreatedAt    string `json:"created_at"`
}

type Client struct {
	Hub      *Hub
	Conn     *websocket.Conn
	Send     chan []byte
	UserID   uint
	DeviceID string
}

type Hub struct {
	Clients    map[uint]map[string]*Client
	Broadcast  chan WSMessage
	Register   chan *Client
	Unregister chan *Client
	Mu         sync.RWMutex
}

func NewHub() *Hub {
	return &Hub{
		Clients:    make(map[uint]map[string]*Client),
		Broadcast:  make(chan WSMessage, 256),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.Mu.Lock()
			if _, ok := h.Clients[client.UserID]; !ok {
				h.Clients[client.UserID] = make(map[string]*Client)
			}
			h.Clients[client.UserID][client.DeviceID] = client
			h.Mu.Unlock()
			log.Printf("Client registered: user=%d, device=%s", client.UserID, client.DeviceID)

		case client := <-h.Unregister:
			h.Mu.Lock()
			if clients, ok := h.Clients[client.UserID]; ok {
				if _, ok := clients[client.DeviceID]; ok {
					delete(clients, client.DeviceID)
					close(client.Send)
					if len(clients) == 0 {
						delete(h.Clients, client.UserID)
					}
				}
			}
			h.Mu.Unlock()
			log.Printf("Client unregistered: user=%d, device=%s", client.UserID, client.DeviceID)

		case message := <-h.Broadcast:
			h.broadcastToUser(message)
		}
	}
}

func (h *Hub) broadcastToUser(message WSMessage) {
	type msgWithUser struct {
		UserID uint
		Data   interface{}
	}

	var userID uint
	if dataMap, ok := message.Data.(map[string]interface{}); ok {
		if uid, ok := dataMap["user_id"].(float64); ok {
			userID = uint(uid)
		}
	}

	if userID == 0 {
		return
	}

	h.Mu.RLock()
	clients, ok := h.Clients[userID]
	h.Mu.RUnlock()

	if !ok {
		return
	}

	msgBytes, err := json.Marshal(message)
	if err != nil {
		log.Println("JSON marshal error:", err)
		return
	}

	fromDevice := message.FromDevice
	for deviceID, client := range clients {
		if deviceID == fromDevice {
			continue
		}
		select {
		case client.Send <- msgBytes:
		default:
			h.Mu.Lock()
			close(client.Send)
			delete(clients, deviceID)
			if len(clients) == 0 {
				delete(h.Clients, userID)
			}
			h.Mu.Unlock()
		}
	}
}

func (h *Hub) SendClipboardSync(userID uint, data ClipboardSyncData, fromDevice string) {
	msg := WSMessage{
		Type:       "clipboard_sync",
		Data:       data,
		Timestamp:  time.Now().Unix(),
		FromDevice: fromDevice,
	}

	msgWithUser := WSMessage{
		Type:      msg.Type,
		Timestamp: msg.Timestamp,
		FromDevice: fromDevice,
		Data: map[string]interface{}{
			"user_id":       userID,
			"id":            data.ID,
			"content":       data.Content,
			"translation":   data.Translation,
			"device_name":   data.DeviceName,
			"is_translated": data.IsTranslated,
			"content_type":  data.ContentType,
			"created_at":    data.CreatedAt,
		},
	}

	h.Broadcast <- msgWithUser
}

func (c *Client) ReadPump() {
	defer func() {
		c.Hub.Unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(65536)
	c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, _, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}
	}
}

func (c *Client) WritePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			c.Conn.WriteMessage(websocket.TextMessage, message)

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func ServeWS(hub *Hub, w http.ResponseWriter, r *http.Request, userID uint, deviceID string) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade error:", err)
		return
	}

	if deviceID == "" {
		deviceID = r.Header.Get("X-Device-ID")
		if deviceID == "" {
			deviceID = "unknown-" + r.RemoteAddr
		}
	}

	client := &Client{
		Hub:      hub,
		Conn:     conn,
		Send:     make(chan []byte, 256),
		UserID:   userID,
		DeviceID: deviceID,
	}

	client.Hub.Register <- client

	go client.WritePump()
	go client.ReadPump()
}
