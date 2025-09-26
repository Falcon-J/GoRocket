package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	_ "image/png"
	"log"
	"math"
	"math/rand/v2"
	"os"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/mp3"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/text"
)

const sampleRate = 44000

var (
	bg           *ebiten.Image
	player       *ebiten.Image
	countdown    *ebiten.Image
	ready_set_go *ebiten.Image
	clouds       *ebiten.Image
	record       *ebiten.Image
	zbutton      *ebiten.Image
	xbutton      *ebiten.Image
	smoke        *ebiten.Image
	myFont       font.Face
	screenWidth  = 480
	screenHeight = 640
	rsg          = 0
	count        = 10

	audioContext *audio.Context
	musicPlayer  *audio.Player

	sfxCountData     []byte
	sfxLaunchData    []byte
	sfxCountDownData []byte
	sfxPowerDownData []byte
	sfxChargeData    []byte
)

type Game struct {
	altitude        float64
	speed           float64
	speedMax        float64
	power           float64
	powerMax        float64
	gravity         float64
	highscore       float64
	saved_highscore float64

	bgoffset float64

	prevKeys    map[ebiten.Key]bool
	launched    bool
	start_count bool
	power_down  bool

	z_down int
	x_down int

	launchSFXPlayer *audio.Player

	rsg_timer   float64
	count_timer float64

	particles []Particle
}

type Particle struct {
	X, Y     float64
	Radius   float64
	Velocity float64
	Opacity  float64
}

func init() {
	var err error
	myFont = loadFont()

	// Import Sounds
	sfxCountData, err = os.ReadFile("assets/sounds/count.mp3")
	if err != nil {
		log.Fatal(err)
	}

	sfxLaunchData, err = os.ReadFile("assets/sounds/launch.mp3")
	if err != nil {
		log.Fatal(err)
	}

	sfxCountDownData, err = os.ReadFile("assets/sounds/countdown.mp3")
	if err != nil {
		log.Fatal(err)
	}

	sfxPowerDownData, err = os.ReadFile("assets/sounds/powerdown.mp3")
	if err != nil {
		log.Fatal(err)
	}

	sfxChargeData, err = os.ReadFile("assets/sounds/charge.mp3")
	if err != nil {
		log.Fatal(err)
	}

	// Load in Artwork
	bg, _, err = ebitenutil.NewImageFromFile("assets/background.png")
	if err != nil {
		log.Fatal(err)
	}

	player, _, err = ebitenutil.NewImageFromFile("assets/player.png")
	if err != nil {
		log.Fatal(err)
	}

	ready_set_go, _, err = ebitenutil.NewImageFromFile("assets/ready_set_go.png")
	if err != nil {
		log.Fatal(err)
	}

	countdown, _, err = ebitenutil.NewImageFromFile("assets/countdown.png")
	if err != nil {
		log.Fatal(err)
	}

	clouds, _, err = ebitenutil.NewImageFromFile("assets/clouds.png")
	if err != nil {
		log.Fatal(err)
	}

	record, _, err = ebitenutil.NewImageFromFile("assets/record.png")
	if err != nil {
		log.Fatal(err)
	}

	zbutton, _, err = ebitenutil.NewImageFromFile("assets/zbutton.png")
	if err != nil {
		log.Fatal(err)
	}

	xbutton, _, err = ebitenutil.NewImageFromFile("assets/xbutton.png")
	if err != nil {
		log.Fatal(err)
	}

	smoke, _, err = ebitenutil.NewImageFromFile("assets/smoke.png")
	if err != nil {
		log.Fatal(err)
	}

}

func playSFX(data []byte) *audio.Player {
	stream, err := mp3.Decode(audioContext, bytes.NewReader(data))
	if err != nil {
		log.Println("Error decoding sound:", err)
		return nil
	}

	sfxPlayer, err := audio.NewPlayer(audioContext, stream)
	if err != nil {
		log.Println("Error creating audio player:", err)
		return nil
	}

	sfxPlayer.Play()
	return sfxPlayer
}

func loadMusic() {
	var err error
	audioContext = audio.NewContext(sampleRate)

	data, err := os.ReadFile("assets/sounds/bossa_nova.mp3")
	if err != nil {
		log.Fatal(err)
	}

	stream, err := mp3.DecodeWithoutResampling(bytes.NewReader(data))
	if err != nil {
		log.Fatal(err)
	}

	musicPlayer, err = audio.NewPlayer(audioContext, stream)
	if err != nil {
		log.Fatal(err)
	}

	musicPlayer.Play()
}

func loadFont() font.Face {
	ttfBytes, err := os.ReadFile("assets/font.ttf")
	if err != nil {
		log.Fatal(err)
	}
	tt, err := opentype.Parse(ttfBytes)
	if err != nil {
		log.Fatal(err)
	}
	const dpi = 72
	face, err := opentype.NewFace(tt, &opentype.FaceOptions{
		Size:    36,
		DPI:     dpi,
		Hinting: font.HintingFull,
	})
	if err != nil {
		log.Fatal(err)
	}
	return face
}

func (g *Game) restartGame() {
	g.altitude = -5763
	g.speed = 0
	g.power = 0
	g.launched = false
	g.start_count = false
	g.power_down = false
	if g.highscore > g.saved_highscore {
		g.saved_highscore = g.highscore
	}
	g.highscore = 0
	rsg = 0
	count = 10
}

func drawTextWithOutline(dst *ebiten.Image, str string, face font.Face, x, y int, textColor, outlineColor color.Color) {
	thickness := 6

	for dx := -thickness; dx <= thickness; dx++ {
		for dy := -thickness; dy <= thickness; dy++ {
			if dx == 0 && dy == 0 {
				continue
			}

			dist := math.Sqrt(float64(dx*dx + dy*dy))
			if dist <= float64(thickness) {
				text.Draw(dst, str, face, x+dx, y+dy, outlineColor)
			}
		}
	}

	// Draw main text
	text.Draw(dst, str, face, x, y, textColor)
}

func (g *Game) JustPressed(key ebiten.Key) bool {
	return ebiten.IsKeyPressed(key) && !g.prevKeys[key]
}

func drawCircle(dst *ebiten.Image, x, y, r float64, clr color.Color) {
	op := &ebiten.DrawImageOptions{}
	scale := r / float64(smoke.Bounds().Dx()/2)
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate(x-r, y-r)
	op.ColorM.ScaleWithColor(clr)
	dst.DrawImage(smoke, op)
}

func (g *Game) Update() error {
	const deltaTime = 1.0 / 60.0

	// Move Clouds
	g.bgoffset -= 30 * deltaTime

	if g.bgoffset <= -float64(screenWidth) {
		g.bgoffset = 0
	}

	// Ready, Set, Go Timer
	if rsg < 3 {
		g.rsg_timer += deltaTime

		if g.rsg_timer >= 1 {
			rsg += 1
			playSFX(sfxCountData)
			g.rsg_timer = 0
		}
	}

	if ebiten.IsKeyPressed(ebiten.KeyZ) {
		g.z_down = 1
	} else {
		g.z_down = 0
	}

	if ebiten.IsKeyPressed(ebiten.KeyX) {
		g.x_down = 1
	} else {
		g.x_down = 0
	}

	if g.launched {
		if g.power <= 0 {
			if g.launchSFXPlayer != nil {
				g.launchSFXPlayer.Pause()
				g.launchSFXPlayer.Rewind()
				g.launchSFXPlayer = nil
			}

			if !g.power_down {
				g.highscore = ((g.altitude * -1) - 5763) * -1
				playSFX(sfxPowerDownData)
				g.power_down = true
			}
		}

	}

	// Draw Particle
	if g.launched && !g.power_down {
		g.particles = append(g.particles, Particle{
			X:        240,
			Y:        550,
			Radius:   16 + rand.Float64()*6,
			Velocity: 100 + rand.Float64()*30,
			Opacity:  1.0,
		})
	}

	for i := 0; i < len(g.particles); i++ {
		p := &g.particles[i]
		p.Y += p.Velocity * deltaTime
		p.Opacity -= 0.015
		p.Radius *= 1.01

		if p.Opacity <= 0 {
			g.particles = append(g.particles[:i], g.particles[i+1:]...)
			i--
		}
	}

	// Countdown Timer
	if rsg >= 3 && !g.launched {
		if count > 0 {
			if !g.start_count {
				playSFX(sfxCountDownData)
				g.start_count = true
			}
			g.count_timer += deltaTime

			if g.count_timer >= 1 {
				count -= 1
				g.count_timer = 0
			}
		} else if count <= 0 {
			if !g.launched {
				g.launchSFXPlayer = playSFX(sfxLaunchData)
				g.launched = true
			}
		}
	}

	// Restarts Launch
	if g.JustPressed(ebiten.KeyR) {
		if g.launched {
			g.restartGame()
		}
	}

	// Charge up Power
	if g.JustPressed(ebiten.KeyZ) {
		if rsg >= 3 && !g.launched {
			g.power += 5
			if g.power > g.powerMax {
				g.power = g.powerMax
			}
			playSFX(sfxChargeData)
		}
	}
	// Charge up Power
	if g.JustPressed(ebiten.KeyX) {
		if rsg >= 3 && !g.launched {
			g.power += 5
			if g.power > g.powerMax {
				g.power = g.powerMax
			}
			playSFX(sfxChargeData)
		}
	}

	if g.launched {
		if g.speed < g.speedMax {
			if g.power > 0 {
				g.speed += 0.6 * deltaTime
				g.power -= 60 * deltaTime
			} else {
				if g.speed > g.gravity && g.altitude >= -5763 {
					g.speed -= 2.4 * deltaTime
				}
			}
		}

		if g.altitude < g.speed && g.altitude >= -5763 {
			g.altitude += g.speed
		}

		if g.altitude < -5763 {
			g.altitude = -5763
			g.restartGame()
		}
	}

	for k := ebiten.Key(0); k <= ebiten.KeyMax; k++ {
		g.prevKeys[k] = ebiten.IsKeyPressed(k)
	}

	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	// Draw Background
	bgOp := &ebiten.DrawImageOptions{}
	bgOp.GeoM.Translate(0, g.altitude)
	screen.DrawImage(bg, bgOp)

	// Draw Clouds
	cloudWidth := clouds.Bounds().Dx()

	cloudsOp := &ebiten.DrawImageOptions{}
	cloudsOp.GeoM.Translate(g.bgoffset, g.altitude+5740)
	screen.DrawImage(clouds, cloudsOp)

	cloudsOp2 := &ebiten.DrawImageOptions{}
	cloudsOp2.GeoM.Translate(g.bgoffset+float64(cloudWidth), g.altitude+5740)
	screen.DrawImage(clouds, cloudsOp2)

	// Draw the Highscore Record
	recordOp := &ebiten.DrawImageOptions{}
	recordOp.GeoM.Translate(0, g.altitude+6015-g.saved_highscore)
	screen.DrawImage(record, recordOp)
	formatHighscore := fmt.Sprintf("%.0fm", g.saved_highscore)
	boundsHighscore := text.BoundString(myFont, formatHighscore)
	textWidthHighscore := boundsHighscore.Dx()
	xHighscore := (screenWidth - textWidthHighscore) / 2

	customColor := color.RGBA{R: 9, G: 27, B: 162, A: 127}
	text.Draw(screen, formatHighscore, myFont, xHighscore, int(g.altitude+6080-g.saved_highscore), customColor)

	// Draw Particles
	for _, p := range g.particles {
		alpha := uint8(p.Opacity * 255)
		col := color.RGBA{255, 255, 255, alpha}
		drawCircle(screen, p.X, p.Y, p.Radius, col)
	}

	// Draw the Player
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(195, 300)
	screen.DrawImage(player, op)

	// Draw Ready, Set, Go
	readyOp := &ebiten.DrawImageOptions{}
	readyOp.GeoM.Translate(float64((screenWidth-303)/2), 130)

	i := rsg
	sx, sy := 0+i*303, 4
	sub := ready_set_go.SubImage(image.Rect(sx, sy, sx+303, sy+118)).(*ebiten.Image)
	screen.DrawImage(sub, readyOp)

	// Draw Countdown
	if rsg >= 3 {
		countOp := &ebiten.DrawImageOptions{}
		countOp.GeoM.Translate(float64((screenWidth-159)/2)-5, 130)

		iCount := count
		c_sx, c_sy := 0+iCount*159, 4
		subCount := countdown.SubImage(image.Rect(c_sx, c_sy, c_sx+159, c_sy+118)).(*ebiten.Image)
		screen.DrawImage(subCount, countOp)
	}

	// Draw Altitude
	altitudeValue := ((g.altitude * -1) - 5763) * -1
	formatAlt := fmt.Sprintf("%.0fm", math.Abs(altitudeValue))
	bounds := text.BoundString(myFont, formatAlt)
	textWidth := bounds.Dx()
	screenWidth := 480
	x := (screenWidth - textWidth) / 2

	//text.Draw(screen, formatAlt, myFont, x, 80, color.White)
	if g.launched {
		drawTextWithOutline(screen, formatAlt, myFont, x, 80, color.White, color.Black)
	} else if g.start_count && !g.launched {
		drawTextWithOutline(screen, "Charge Your Rocket!", myFont, 60, 80, color.White, color.Black)

		// Draw Z Button
		zOp := &ebiten.DrawImageOptions{}
		zOp.GeoM.Translate(280, 270)
		zi := g.z_down
		zsx, zsy := 0+zi*135, 2
		zSub := zbutton.SubImage(image.Rect(zsx, zsy, zsx+135, zsy+135)).(*ebiten.Image)
		screen.DrawImage(zSub, zOp)

		// Draw X Button
		xOp := &ebiten.DrawImageOptions{}
		xOp.GeoM.Translate(310, 340)
		xi := g.x_down
		xsx, xsy := 0+xi*135, 2
		xSub := xbutton.SubImage(image.Rect(xsx, xsy, xsx+135, xsy+135)).(*ebiten.Image)
		screen.DrawImage(xSub, xOp)
	}

	// Draw Power Meter
	if (rsg >= 3 && !g.power_down) || g.launched {
		barWidth := 300.0
		barHeight := 20.0
		barX := (float64(screenWidth) - barWidth) / 2
		barY := 560.0

		ebitenutil.DrawRect(screen, barX, barY, barWidth, barHeight, color.RGBA{0, 0, 0, 180})

		if g.powerMax > 0 {
			fill := barWidth - 4
			percent := g.power / g.powerMax
			if percent > 1 {
				percent = 1
			}
			if percent < 0 {
				percent = 0
			}
			fillWidth := fill * percent
			ebitenutil.DrawRect(screen, barX+2, barY+2, fillWidth, barHeight-4, color.RGBA{255, 165, 0, 255})

			label := fmt.Sprintf("Fuel %3.0f%%", percent*100)
			bounds := text.BoundString(myFont, label)
			textX := int(barX + (barWidth-float64(bounds.Dx()))/2)
			textY := int(barY) + int(barHeight) - 4
			drawTextWithOutline(screen, label, myFont, textX, textY, color.White, color.Black)
		}
	}

}

func (g *Game) Layout(outsideWidth, outsideHeight int) (w, h int) {
	return screenWidth, screenHeight
}

func main() {
	loadMusic()

	ebiten.SetWindowSize(screenWidth, screenHeight)
	ebiten.SetWindowTitle("Go Game")

	game := &Game{
		highscore:       0,
		altitude:        -5763,
		speed:           0,
		speedMax:        20,
		power:           0,
		powerMax:        600,
		gravity:         -40,
		start_count:     false,
		power_down:      false,
		saved_highscore: -9999,
		prevKeys:        make(map[ebiten.Key]bool),
	}

	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
