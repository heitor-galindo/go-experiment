package main

import (
	"bytes"
	"encoding/json"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"sync"
	"time"
)

type PlayerState string

const (
	Online  PlayerState = "online"
	Offline PlayerState = "offline"
)

type PlayerPosition struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type Player struct {
	ID         int            `json:"id"`
	Color      string         `json:"color"`
	Position   PlayerPosition `json:"position"`
	State      PlayerState    `json:"state"`
	UdpAddress *net.UDPAddr   `json:"udpAddress"`
	LastSeen   time.Time
}

type Movement struct {
	ID int `json:"id"`
	X  int `json:"x"`
	Y  int `json:"y"`
}

var (
	globalPlayers []*Player
	mu            sync.RWMutex
)

var OnlinePlayerPosition []Movement

func getOnlinePlayerPosition() {
	mu.RLock()
	defer mu.RUnlock()
	OnlinePlayerPosition = OnlinePlayerPosition[:0]
	for _, p := range globalPlayers {
		if p.State == Online {
			position := Movement{p.ID, p.Position.X, p.Position.Y}
			OnlinePlayerPosition = append(OnlinePlayerPosition, position)
		}
	}
}

func (player *Player) findPlayer() *Player {
	slog.Debug("Searching player", "player_id", player.ID)
	mu.RLock()
	defer mu.RUnlock()

	for _, p := range globalPlayers {
		if p.ID == player.ID {
			slog.Debug("Player found.", "player_id", player.ID)
			return p
		}
	}
	slog.Debug("Player not found.", "player_id", player.ID)
	return nil
}

func login(w http.ResponseWriter, req *http.Request) {
	w.Header().Add("Content-Type", "application/json")

	var p Player
	err := json.NewDecoder(req.Body).Decode(&p)
	if err != nil {
		slog.Error("Login error.", "login_error", err, "player_id", p.ID)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid json payload."})
		return
	}

	player := p.findPlayer()

	if player == nil {
		slog.Debug("Adding new player.", "player_id", p.ID)

		mu.Lock()
		p.State = Offline
		p.LastSeen = time.Now()
		newPlayer := p
		globalPlayers = append(globalPlayers, &newPlayer)
		mu.Unlock()
		player = &newPlayer
	}

	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(player)
	if err != nil {
		slog.Error("Failed to encode login message.", "player_id", player.ID)
		return
	}

	slog.Debug("New player joined.", "player_id", player.ID)
}

func logout(w http.ResponseWriter, req *http.Request) {
	w.Header().Add("Content-Type", "application/json")

	var p Player
	err := json.NewDecoder(req.Body).Decode(&p)
	if err != nil {
		slog.Error("Logout error.", "logout_error", err, "player_id", p.ID)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "invalid json payload."})
		return
	}

	player := p.findPlayer()

	mu.Lock()
	if player != nil && player.State == Online {
		player.State = Offline
		player.LastSeen = time.Now()
		mu.Unlock()

		slog.Debug("Player logged out.", "player_id", player.ID)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "logged out."})

	} else {
		mu.Unlock()

		slog.Debug("Player not logged.", "player_id", p.ID)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"status": "you are not logged."})
	}
}

func dropOfflinePlayers(offlinePlayers []*Player) {
	mu.Lock()
	defer mu.Unlock()
	for _, p := range offlinePlayers {
		p.State = Offline
	}
}

func broadcastPosition(conn *net.UDPConn) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		getOnlinePlayerPosition()

		mu.RLock()
		udpPayload, err := json.Marshal(OnlinePlayerPosition)
		globalPlayersCopy := make([]*Player, len(globalPlayers))
		copy(globalPlayersCopy, globalPlayers)
		mu.RUnlock()

		if err != nil {
			slog.Error("Error while enconding payload", "error", err)
		}

		var offlinePlayers []*Player
		for _, p := range globalPlayersCopy {
			if time.Since(p.LastSeen) > 10*time.Second {
				offlinePlayers = append(offlinePlayers, p)
				continue
			}
			if p.UdpAddress == nil {
				continue
			}
			_, err = conn.WriteToUDP(udpPayload, p.UdpAddress)
			if err != nil {
				offlinePlayers = append(offlinePlayers, p)
			}
		}
		if len(offlinePlayers) > 0 {
			dropOfflinePlayers(offlinePlayers)
		}
	}
}

func startUDPServer(conn *net.UDPConn) {
	defer conn.Close()

	buffer := make([]byte, 1024)
	for {
		n, clientAddr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			log.Printf("Read error: %v", err)
			continue
		}

		bufferData := bytes.NewBuffer(buffer[:n])
		var playerMovement Movement
		err = json.NewDecoder(bufferData).Decode(&playerMovement)
		if err != nil {
			log.Panicf("Error while decoding movement message: %v", err)
		}

		p := Player{ID: playerMovement.ID}
		player := p.findPlayer()

		if player != nil {
			mu.Lock()
			player.UdpAddress = clientAddr
			player.Position.X = playerMovement.X
			player.Position.Y = playerMovement.Y
			player.LastSeen = time.Now()
			player.State = Online
			mu.Unlock()
		}
	}
}

func main() {
	opts := &slog.HandlerOptions{
		Level: slog.LevelDebug,
		// Level: slog.LevelInfo
	}
	handler := slog.NewTextHandler(os.Stdout, opts)
	logger := slog.New(handler)
	slog.SetDefault(logger)

	// UDP SERVER
	log.Print("Starting UDP Server")
	addr, err := net.ResolveUDPAddr("udp", ":8091")
	if err != nil {
		log.Fatal("Couldnt resolve address. Err: ", err)
	}
	udpConn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Fatal("UDP listen failed: ", err)
	}

	go startUDPServer(udpConn)
	go broadcastPosition(udpConn)

	log.Print("Starting TCP server")
	http.HandleFunc("/login", login)
	http.HandleFunc("/logout", logout)
	http.ListenAndServe(":8090", nil) // isso inicia um net.Listen(tcp)

}
