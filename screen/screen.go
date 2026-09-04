package main

import (
	"errors"
	"fmt"
	"image/color"
	"log"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

var pointerImage = ebiten.NewImage(8, 8)

func init() {
	pointerImage.Fill(color.RGBA{0xff, 0, 0, 0xff})
}

const (
	screenWidth  = 640
	screenHeight = 480
	padding      = 40
	tileSize     = 40
)

type Game struct {
	x float64
	y float64
}

func (g *Game) Update() error {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		return errors.New("game ended by player")
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButton0) {
		dx, dy := ebiten.CursorPosition()
		g.x = float64(dx)
		g.y = float64(dy)
	}
	g.handleMovement()
	return nil
}

func border(screen *ebiten.Image, padding int, width, height int) {
	vector.StrokeLine(screen, float32(width-padding), float32(height-padding), float32(width-padding), float32(padding), 1, color.RGBA{11, 156, 49, 255}, false)
	vector.StrokeLine(screen, float32(padding), float32(height-padding), float32(padding), float32(padding), 1, color.RGBA{11, 156, 49, 255}, false)
	vector.StrokeLine(screen, float32(padding), float32(height-padding), float32(width-padding), float32(height-padding), 1, color.RGBA{11, 156, 49, 255}, false)
	vector.StrokeLine(screen, float32(padding), float32(padding), float32(width-padding), float32(padding), 1, color.RGBA{11, 156, 49, 255}, false)
}

func grid(screen *ebiten.Image, tileSize int, width, height int) {
	for x := range width {
		xPosition := tileSize * x
		if xPosition > tileSize && xPosition < width {
			vector.StrokeLine(screen, float32(xPosition), float32(height-tileSize), float32(xPosition), float32(tileSize), 1, color.RGBA{11, 156, 49, 255}, false)
			for y := range height {
				yPosition := tileSize * y
				if yPosition > tileSize && yPosition < height {
					vector.StrokeLine(screen, float32(xPosition), float32(yPosition), float32(tileSize), float32(yPosition), 1, color.RGBA{11, 156, 49, 255}, false)
				}
			}
		}
	}
}

func (g *Game) handleMovement() {
	if ebiten.IsKeyPressed(ebiten.KeyW) {
		g.y -= 4
	}
	if ebiten.IsKeyPressed(ebiten.KeyS) {
		g.y += 4
	}
	if ebiten.IsKeyPressed(ebiten.KeyA) {
		g.x -= 4
	}
	if ebiten.IsKeyPressed(ebiten.KeyD) {
		g.x += 4
	}
	if g.x < padding {
		g.x = padding - 1
	}
	if g.x > screenWidth-padding {
		g.x = screenWidth - padding - 1
	}
	if g.y < padding {
		g.y = padding - 1
	}
	if g.y > screenHeight-padding {
		g.y = screenHeight - padding - 1
	}
}
func (g *Game) Draw(screen *ebiten.Image) {

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(g.x, g.y)
	screen.DrawImage(pointerImage, op)
	border(screen, padding, screenWidth, screenHeight)
	grid(screen, padding, screenWidth, screenHeight)
	ebitenutil.DebugPrint(screen, fmt.Sprintf("Move the red point by mouse \n(%0.2f, %0.2f)", g.x, g.y))
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenWidth, screenHeight
}

func main() {
	g := &Game{x: screenWidth / 2, y: screenHeight / 2}
	ebiten.SetWindowSize(screenWidth, screenHeight)
	ebiten.SetWindowTitle("Game map")
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}
