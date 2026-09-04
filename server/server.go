package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math/rand/v2"
	"net"
	"net/http"
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
func updatePlayerPosition(m Movement, conn *net.UDPAddr){
	player := findPlayerById(m.ID)
	if player != nil {
		mu.Lock()
		player.Position.X = m.X
		player.Position.Y = m.Y
		player.UdpAddress = conn
		mu.Unlock()
	}
	log.Print("Player position updated")
}

func findPlayerById(id int) *Player {
	log.Print("Searching for player: ", id)
	mu.RLock()
	defer mu.RUnlock()

	for _, p := range globalPlayers {
		if p.ID == id {
			log.Printf("Player %v found", id)
			return p
		}
	}
	log.Print("Player not found")
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
	log.Printf("Online players: %v", count)
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

func login(w http.ResponseWriter, req *http.Request) {
	var p Player

	err := json.NewDecoder(req.Body).Decode(&p)
	if err != nil {
		log.Panicf("Fail decode player in login error: %v", err)
	}

	player := findPlayerById(p.ID)
	if player != nil {
		mu.Lock()
		player.State = "online"
		mu.Unlock()
	} else {
		player = createNewPlayer(p)
	}

	// send player information to client
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(200)
	jsonData, _ := json.Marshal(player)
	fmt.Fprintf(w, "%+v\n", bytes.NewBuffer(jsonData))

	log.Printf("New player joined: %v", player.ID)
	countOnlinePlayers()
}

func logout(w http.ResponseWriter, req *http.Request) {
	var id int
	err := json.NewDecoder(req.Body).Decode(&id)
	if err != nil {
		log.Panic(err)
	}

	p := findPlayerById(id)
	if p != nil && p.State == "online" {
		mu.Lock()
		p.State = "offline"
		mu.Unlock()
	}
	log.Printf("Player ID %v left the game", id)

	w.WriteHeader(200)
	w.Header().Add("Content-Type", "application/json")
	jsonData, _ := json.Marshal("logged out")
	fmt.Fprintf(w, "%v\n", bytes.NewBuffer(jsonData))
	countOnlinePlayers()
}

func logoutFromUdp(id int) {
	p := findPlayerById(id)
	if p != nil && p.State == "online" {
		mu.Lock()
		p.State = "offline"
		mu.Unlock()
	}
	log.Printf("Player ID %v left the game", id)

}

func startUDPServer(conn *net.UDPConn) {
	defer conn.Close()

	buffer := make([]byte, 1024)
	for {
		n, clientAddr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			log.Printf("Read error: %v", err)
		}
		log.Print(clientAddr.String())
		
		bufferData := bytes.NewBuffer(buffer[:n])
		var playerMovement Movement
		err = json.NewDecoder(bufferData).Decode(&playerMovement)
		if err != nil {
			log.Panicf("Error while decoding movement message: %v", err)
		}

		updatePlayerPosition(playerMovement, clientAddr)
		getOnlinePlayerPosition()
		log.Printf("Online player positions: %v\n,", OnlinePlayerPosition)
		udpPayload, err := json.Marshal(OnlinePlayerPosition)
		conn.WriteToUDP(udpPayload, clientAddr)
		if err != nil {
			log.Printf("Error to send udp message: %v", err)
		}
	}
	
}

func main() {

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
