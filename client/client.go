package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"time"
)

const serverTCPAddress = "http://100.66.46.4:8090"
const serverUDPAddress = "100.66.46.4:8091"
const contentType = "application/json"

type PlayerPosition struct {
	X int `json:"x"`
	Y int `json:"y"`
}

type PlayerState string

const (
	Online  PlayerState = "online"
	Offline PlayerState = "offline"
)

type Player struct {
	ID         int            `json:"id"`
	Color      string         `json:"color"`
	Position   PlayerPosition `json:"position"`
	UdpAddress *net.UDPAddr   `json:"udpAddress"`
	State      PlayerState    `json:"state"`
}

type Movement struct {
	ID int `json:"id"`
	X  int `json:"x"`
	Y  int `json:"y"`
}

var player *Player
var players []Player

func initalize() *Player {

	r := rand.IntN(256)
	g := rand.IntN(256)
	b := rand.IntN(256)
	hexColor := fmt.Sprintf("#%02X%02X%02X", r, g, b)

	return &Player{
		ID:         rand.IntN(1000),
		Position:   PlayerPosition{0, 0},
		State:      Offline,
		Color:      hexColor,
		UdpAddress: nil,
	}
}

func (player *Player) login() {
	path, err := url.JoinPath(serverTCPAddress, "login")
	if err != nil {
		log.Panic(err)
	}
	jsonData, err := json.Marshal(player)
	if err != nil {
		log.Panic(err)
	}

	log.Printf("Login player: %v", player.ID)
	resp, err := http.Post(path, contentType, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Panic(err)
	}

	defer func() {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()

	if resp.StatusCode == http.StatusOK {
		err := json.NewDecoder(resp.Body).Decode(&player)
		if err != nil {
			log.Panic(err)
		}
	}
}

func (player *Player) logout() {
	path, err := url.JoinPath(serverTCPAddress, "logout")
	if err != nil {
		log.Panic(err)
	}
	jsonData, err := json.Marshal(player)
	if err != nil {
		log.Panic(err)
	}
	resp, err := http.Post(path, contentType, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Panic(err)
	}

	defer func() {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()

	var logoutResp map[string]string
	err = json.NewDecoder(resp.Body).Decode(&logoutResp)
	if err != nil {
		log.Print(err)
	}
	if resp.StatusCode != http.StatusOK {
		log.Printf("Error while login out: %v", logoutResp["status"])
		return
	}
	log.Print(logoutResp["status"])
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
	log.Printf("New posiition: %v", movement)

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
	player := initalize()
	player.login()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		player.logout()
		os.Exit(0)
	}()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		sendPosition(player)
	}

	// player.logout() // teste manual
}
