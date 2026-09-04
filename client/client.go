package main

import (
	"bytes"
	"encoding/json"
	"log"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"time"
)

const serverTCPAddress = "http://100.66.46.4:8090"
const serverUDPAddress = "100.66.46.4:8091"
const contentType = "application/json"

type PlayerPosition struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type Player struct {
	ID         int            `json:"id"`
	Color      string         `json:"color"`
	Position   PlayerPosition `json:"position"`
	UdpAddress *net.UDPAddr   `json:"udpAddress"`
}

type Movement struct {
	ID int `json:"id"`
	X  int `json:"x"`
	Y  int `json:"y"`
}

var player *Player
var players []Player

func getLocalUDPPort() *net.UDPAddr {
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		log.Panic(err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Panic(err)
	}
	defer conn.Close()

	return conn.LocalAddr().(*net.UDPAddr)
}

func initalizePlayer() *Player {
	return &Player{
		ID:         rand.IntN(1000),
		Position:   PlayerPosition{0, 0},
		UdpAddress: getLocalUDPPort(),
	}
}

func login(p *Player) {
	path, err := url.JoinPath(serverTCPAddress, "login")
	if err != nil {
		log.Panic(err)
	}
	jsonData, err := json.Marshal(p)
	if err != nil {
		log.Panic(err)
	}

	log.Printf("Login player: %v", p.ID)
	resp, err := http.Post(path, contentType, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Panic(err)
	}

	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		err := json.NewDecoder(resp.Body).Decode(&player)
		if err != nil {
			log.Panic(err)
		}
	}
}

func logout(p *Player) {
	path, err := url.JoinPath(serverTCPAddress, "logout")
	if err != nil {
		log.Panic(err)
	}
	jsonData, err := json.Marshal(p)
	if err != nil {
		log.Panic(err)
	}
	resp, err := http.Post(path, contentType, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Panic(err)
	}
	defer resp.Body.Close()
	var logoutResp string

	if resp.StatusCode == 200 {
		err := json.NewDecoder(resp.Body).Decode(&logoutResp)
		if err != nil {
			log.Panic(err)
		}
	}
	log.Printf("%+v", &logoutResp)

}

func sendPosition(p *Player) {
	addr, err := net.ResolveUDPAddr("udp", serverUDPAddress)
	if err != nil {
		log.Fatal("Couldn’t resolve address:", err)
	}
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		log.Fatal("Connection failed:", err)
	}
	defer conn.Close()

	p.Position = PlayerPosition{X: rand.IntN(99), Y: rand.IntN(99)}
	movement := Movement{ID: p.ID, X: p.Position.X, Y: p.Position.Y}

	message, err := json.Marshal(movement)
	_, err = conn.Write(message)
	if err != nil {
		log.Printf("Send failed: %v", err)
		return
	}

	buffer := make([]byte, 1024)
	n, _, err := conn.ReadFromUDP(buffer)
	if err != nil {
		log.Printf("Receive error: %v", err)
		return
	}
	log.Printf("Server message: %s\n", string(buffer[:n]))
}

func main() {
	player = initalizePlayer()
	login(player)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		sendPosition(player)
	}

	// logout(player)
}
