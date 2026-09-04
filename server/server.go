package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"math/rand/v2"
	"net"
	"net/http"
	"os"
	"sync"
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

func playerGenerator(p Player) *Player {
	r := rand.IntN(256)
	g := rand.IntN(256)
	b := rand.IntN(256)
	hexColor := fmt.Sprintf("#%02X%02X%02X", r, g, b)
	return &Player{
		ID:         p.ID,
		Color:      hexColor,
		Position:   p.Position,
		UdpAddress: p.UdpAddress,
		State:      "online",
	}
}

func getOnlinePlayerPosition() {
	mu.RLock()
	defer mu.RUnlock()
	OnlinePlayerPosition = OnlinePlayerPosition[:0]
	for _, p := range globalPlayers {
		if p.State == "online" {
			position := Movement{p.ID, p.Position.X, p.Position.Y}
			OnlinePlayerPosition = append(OnlinePlayerPosition, position)
		}
	}
}

func (player *Player) updatePosition(m Movement) {
	mu.Lock()
	player.Position.X = m.X
	player.Position.Y = m.Y
	mu.Unlock()
	slog.Debug("Player position updated.", "player_id", player.ID)
}

func updatePlayerPosition(m Movement, conn *net.UDPAddr) {
	player := findPlayerById(m.ID)
	if player != nil {
		mu.Lock()
		player.Position.X = m.X
		player.Position.Y = m.Y
		player.UdpAddress = conn
		mu.Unlock()
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

func findPlayerById(id int) *Player {
	mu.RLock()
	defer mu.RUnlock()
	for _, p := range globalPlayers {
		if p.ID == id {
			return p
		}
	}
	return nil
}

func countOnlinePlayers() {
	mu.RLock()
	defer mu.RUnlock()

	count := 0
	for _, p := range globalPlayers {
		if p.State == "online" {
			count++
		}
	}
	slog.Debug("Online players", "player_count", count)
}

func createNewPlayer(p Player) *Player {
	log.Printf("Creating new player... ID: %v", p.ID)
	player := playerGenerator(p)

	mu.Lock()
	defer mu.Unlock()
	globalPlayers = append(globalPlayers, player)
	log.Print("New player created!")

	return player
}

func (player *Player) changeState(state PlayerState) {
	mu.Lock()
	player.State = state
	mu.Unlock()
}

func login(w http.ResponseWriter, req *http.Request) {

	var p Player
	err := json.NewDecoder(req.Body).Decode(&p)
	if err != nil {
		slog.Error("Login error.", "login_error", err, "player_id", p.ID)
	}

	player := p.findPlayer()
	if player != nil {
		player.changeState(Online)
	} else {
		player = createNewPlayer(p)
	}

	// send player information to client
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(200)
	jsonData, _ := json.Marshal(player)
	fmt.Fprintf(w, "%+v\n", bytes.NewBuffer(jsonData))

	slog.Debug("New player joined.", "player_id", player.ID)
	countOnlinePlayers()
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

func startUDPServer(conn *net.UDPConn) {
	defer conn.Close()

	buffer := make([]byte, 1024)
	// loop UDP
	for {
		n, clientAddr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			log.Printf("Read error: %v", err)
		}

		bufferData := bytes.NewBuffer(buffer[:n])
		var playerMovement Movement
		err = json.NewDecoder(bufferData).Decode(&playerMovement)
		if err != nil {
			log.Panicf("Error while decoding movement message: %v", err)
		}

		player := Player{ID: playerMovement.ID}
		player.findPlayer()
		if player.State == Online {
			player.updatePosition(playerMovement)
		}

		getOnlinePlayerPosition()
		udpPayload, err := json.Marshal(OnlinePlayerPosition)
		conn.WriteToUDP(udpPayload, clientAddr)
		if err != nil {
			log.Printf("Error to send udp message: %v", err)
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

	log.Print("Starting TCP server")
	http.HandleFunc("/login", login)
	http.HandleFunc("/logout", logout)
	http.ListenAndServe(":8090", nil) // isso iniciar um net.Listen(tcp)

}
