package orderbook_service

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const wsGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

// WSMessage is the JSON envelope sent over /api/v1/orderbook/ws.
type WSMessage struct {
	Channel string      `json:"channel"` // "depth" | "trades" | "orders" | "system"
	Pair    string      `json:"pair,omitempty"`
	Address string      `json:"address,omitempty"`
	Data    interface{} `json:"data"`
	Ts      int64       `json:"ts"`
}

type wsClientSub struct {
	op      string // "subscribe" | "unsubscribe" | "ping"
	channel string // "depth" | "trades" | "orders"
	pair    string
	address string
}

type wsClient struct {
	conn     net.Conn
	br       *bufio.Reader
	writeMu  sync.Mutex
	subMu    sync.RWMutex
	subs     map[string]struct{} // e.g. "depth:<pair>", "trades:<pair>", "orders:<addr>"
	closed   chan struct{}
	closeOne sync.Once
}

func (c *wsClient) close() {
	c.closeOne.Do(func() {
		close(c.closed)
		_ = c.conn.Close()
	})
}

func (c *wsClient) subscribe(topic string) {
	c.subMu.Lock()
	c.subs[strings.ToLower(topic)] = struct{}{}
	c.subMu.Unlock()
}

func (c *wsClient) unsubscribe(topic string) {
	c.subMu.Lock()
	delete(c.subs, strings.ToLower(topic))
	c.subMu.Unlock()
}

func (c *wsClient) hasTopic(topic string) bool {
	c.subMu.RLock()
	_, ok := c.subs[strings.ToLower(topic)]
	c.subMu.RUnlock()
	return ok
}

func (c *wsClient) writeFrame(opcode byte, payload []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_ = c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))

	var hdr [10]byte
	hdr[0] = 0x80 | (opcode & 0x0f) // FIN=1
	n := len(payload)
	var hlen int
	switch {
	case n <= 125:
		hdr[1] = byte(n)
		hlen = 2
	case n <= 65535:
		hdr[1] = 126
		binary.BigEndian.PutUint16(hdr[2:4], uint16(n))
		hlen = 4
	default:
		hdr[1] = 127
		binary.BigEndian.PutUint64(hdr[2:10], uint64(n))
		hlen = 10
	}
	if _, err := c.conn.Write(hdr[:hlen]); err != nil {
		return err
	}
	if n > 0 {
		_, err := c.conn.Write(payload)
		return err
	}
	return nil
}

func (c *wsClient) readFrame() (byte, []byte, error) {
	hdr := make([]byte, 2)
	if _, err := io.ReadFull(c.br, hdr); err != nil {
		return 0, nil, err
	}
	opcode := hdr[0] & 0x0f
	masked := (hdr[1] & 0x80) != 0
	plen := uint64(hdr[1] & 0x7f)

	switch plen {
	case 126:
		var ext [2]byte
		if _, err := io.ReadFull(c.br, ext[:]); err != nil {
			return 0, nil, err
		}
		plen = uint64(binary.BigEndian.Uint16(ext[:]))
	case 127:
		var ext [8]byte
		if _, err := io.ReadFull(c.br, ext[:]); err != nil {
			return 0, nil, err
		}
		plen = binary.BigEndian.Uint64(ext[:])
	}
	if plen > 1<<20 {
		return 0, nil, errors.New("websocket frame too large")
	}

	var maskKey [4]byte
	if masked {
		if _, err := io.ReadFull(c.br, maskKey[:]); err != nil {
			return 0, nil, err
		}
	}
	payload := make([]byte, plen)
	if plen > 0 {
		if _, err := io.ReadFull(c.br, payload); err != nil {
			return 0, nil, err
		}
		if masked {
			for i := range payload {
				payload[i] ^= maskKey[i%4]
			}
		}
	}
	return opcode, payload, nil
}

// WSHub manages zero-dependency RFC 6455 WebSocket connections and topic broadcasts.
type WSHub struct {
	mu      sync.RWMutex
	clients map[*wsClient]struct{}
}

func NewWSHub() *WSHub {
	return &WSHub{
		clients: make(map[*wsClient]struct{}),
	}
}

func (h *WSHub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

func (h *WSHub) Broadcast(topic string, msg WSMessage) {
	if msg.Ts == 0 {
		msg.Ts = time.Now().UnixMilli()
	}
	raw, err := json.Marshal(msg)
	if err != nil {
		return
	}
	h.mu.RLock()
	targets := make([]*wsClient, 0)
	for c := range h.clients {
		if c.hasTopic(topic) || c.hasTopic(msg.Channel+":*") {
			targets = append(targets, c)
		}
	}
	h.mu.RUnlock()

	for _, c := range targets {
		if err := c.writeFrame(0x1, raw); err != nil {
			h.removeClient(c)
		}
	}
}

func (h *WSHub) removeClient(c *wsClient) {
	h.mu.Lock()
	delete(h.clients, c)
	h.mu.Unlock()
	c.close()
}

// ServeHTTP upgrades an HTTP request to RFC 6455 WebSocket and handles subscribe/ping messages.
func (h *WSHub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		http.Error(w, "Upgrade: websocket required", http.StatusUpgradeRequired)
		return
	}
	key := strings.TrimSpace(r.Header.Get("Sec-WebSocket-Key"))
	if key == "" {
		http.Error(w, "missing Sec-WebSocket-Key", http.StatusBadRequest)
		return
	}
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "websocket hijack unsupported", http.StatusInternalServerError)
		return
	}
	conn, rw, err := hj.Hijack()
	if err != nil {
		return
	}

	sum := sha1.Sum([]byte(key + wsGUID))
	accept := base64.StdEncoding.EncodeToString(sum[:])

	resp := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + accept + "\r\n\r\n"
	if _, err := conn.Write([]byte(resp)); err != nil {
		_ = conn.Close()
		return
	}

	client := &wsClient{
		conn:   conn,
		br:     rw.Reader,
		subs:   make(map[string]struct{}),
		closed: make(chan struct{}),
	}

	// Support query-string auto-subscribe: ?pair=<pairKey>&address=<addr>
	if pair := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("pair"))); pair != "" {
		client.subscribe("depth:" + pair)
		client.subscribe("trades:" + pair)
	}
	if addr := strings.TrimSpace(r.URL.Query().Get("address")); addr != "" {
		client.subscribe("orders:" + strings.ToLower(addr))
	}

	h.mu.Lock()
	h.clients[client] = struct{}{}
	h.mu.Unlock()

	welcome, _ := json.Marshal(WSMessage{
		Channel: "system",
		Data:    map[string]string{"status": "connected"},
		Ts:      time.Now().UnixMilli(),
	})
	_ = client.writeFrame(0x1, welcome)

	go h.readLoop(client)
}

func (h *WSHub) readLoop(c *wsClient) {
	defer h.removeClient(c)
	for {
		_ = c.conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		op, payload, err := c.readFrame()
		if err != nil {
			return
		}
		switch op {
		case 0x8: // close
			_ = c.writeFrame(0x8, nil)
			return
		case 0x9: // ping
			_ = c.writeFrame(0xA, payload)
		case 0x1: // text
			var req struct {
				Op      string `json:"op"`
				Channel string `json:"channel"`
				Pair    string `json:"pair"`
				Address string `json:"address"`
			}
			if err := json.Unmarshal(payload, &req); err != nil {
				continue
			}
			switch strings.ToLower(req.Op) {
			case "ping":
				pong, _ := json.Marshal(WSMessage{Channel: "system", Data: map[string]string{"op": "pong"}, Ts: time.Now().UnixMilli()})
				_ = c.writeFrame(0x1, pong)
			case "subscribe":
				if topic := buildTopic(req.Channel, req.Pair, req.Address); topic != "" {
					c.subscribe(topic)
				}
			case "unsubscribe":
				if topic := buildTopic(req.Channel, req.Pair, req.Address); topic != "" {
					c.unsubscribe(topic)
				}
			}
		}
	}
}

func buildTopic(channel, pair, addr string) string {
	ch := strings.ToLower(strings.TrimSpace(channel))
	switch ch {
	case "depth", "trades":
		p := strings.ToLower(strings.TrimSpace(pair))
		if p == "" {
			p = "*"
		}
		return ch + ":" + p
	case "orders":
		a := strings.ToLower(strings.TrimSpace(addr))
		if a == "" {
			a = "*"
		}
		return ch + ":" + a
	default:
		return ""
	}
}
