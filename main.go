package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	_ "image/png"
	"io"
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

const (
	sampleRate    = 44000
	highscoreFile = "highscore.json"
	comboTimeout  = 0.35
	basePowerGain = 5.0
	comboBonus    = 1.5
)

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

	voiceSamples map[int][]byte
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
	peakSpeed       float64
	totalFuel       float64
	prepDuration    float64
	runDuration     float64
	comboTimer      float64
	comboCount      int
	maxCombo        int
	tapCount        int
	lastComboKey    ebiten.Key
	gameOver        bool

	bgoffset float64

	prevKeys    map[ebiten.Key]bool
	launched    bool
	start_count bool
	power_down  bool

	z_down int
	x_down int

	launchSFXPlayer *audio.Player
	voicePlayer     *audio.Player

	rsg_timer   float64
	count_timer float64

	shakeTimer     float64
	shakeDuration  float64
	shakeMagnitude float64
	shakeOffsetX   float64
	shakeOffsetY   float64

	particles []Particle

	lastResult ResultStats
}

type Particle struct {
	X, Y     float64
	Radius   float64
	Velocity float64
	Opacity  float64
}

type ResultStats struct {
	Altitude      float64
	PeakSpeed     float64
	Duration      float64
	PrepDuration  float64
	TapCount      int
	MaxCombo      int
	AverageTPS    float64
	FuelCollected float64
}

type pcmStream struct {
	data []byte
	pos  int64
}

func newPCMStream(data []byte) *pcmStream {
	return &pcmStream{data: data}
}

func (p *pcmStream) Read(b []byte) (int, error) {
	if p.pos >= int64(len(p.data)) {
		return 0, io.EOF
	}
	n := copy(b, p.data[p.pos:])
	p.pos += int64(n)
	return n, nil
}

func (p *pcmStream) Seek(offset int64, whence int) (int64, error) {
	var newPos int64
	switch whence {
	case io.SeekStart:
		newPos = offset
	case io.SeekCurrent:
		newPos = p.pos + offset
	case io.SeekEnd:
		newPos = int64(len(p.data)) + offset
	default:
		return p.pos, fmt.Errorf("invalid whence: %d", whence)
	}
	if newPos < 0 {
		return p.pos, fmt.Errorf("invalid seek position")
	}
	p.pos = newPos
	return p.pos, nil
}

func (p *pcmStream) Close() error {
	return nil
}

func generateVoiceSample(number int) []byte {
	duration := 0.45
	samples := int(float64(sampleRate) * duration)
	data := make([]byte, samples*2)
	freq := 260.0 + float64(number)*22.0
	if number == 0 {
		freq = 220.0
	}
	for i := 0; i < samples; i++ {
		t := float64(i) / float64(sampleRate)
		env := math.Exp(-3.4 * t / duration)
		value := math.Sin(2 * math.Pi * freq * t)
		sample := int16(value * env * 32767)
		binary.LittleEndian.PutUint16(data[i*2:], uint16(sample))
	}
	return data
}

func loadVoiceSamples() {
	voiceSamples = make(map[int][]byte)
	for i := 0; i <= 10; i++ {
		voiceSamples[i] = generateVoiceSample(i)
	}
}

func playVoiceClip(number int) *audio.Player {
	if audioContext == nil {
		return nil
	}
	clip, ok := voiceSamples[number]
	if !ok {
		return nil
	}
	player, err := audio.NewPlayer(audioContext, newPCMStream(clip))
	if err != nil {
		log.Println("error playing voice clip:", err)
		return nil
	}
	player.Play()
	return player
}

func init() {
	var err error
	myFont = loadFont()
	loadVoiceSamples()

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
	g.gameOver = false
	g.highscore = 0
	g.totalFuel = 0
	g.tapCount = 0
	g.maxCombo = 0
	g.comboCount = 0
	g.comboTimer = 0
	g.lastComboKey = 0
	g.prepDuration = 0
	g.runDuration = 0
	g.peakSpeed = 0
	g.shakeTimer = 0
	g.shakeOffsetX = 0
	g.shakeOffsetY = 0
	if g.launchSFXPlayer != nil {
		g.launchSFXPlayer.Close()
		g.launchSFXPlayer = nil
	}
	if g.voicePlayer != nil {
		g.voicePlayer.Close()
		g.voicePlayer = nil
	}
	for k := range g.prevKeys {
		g.prevKeys[k] = false
	}
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

func (g *Game) currentAltitude() float64 {
	return ((g.altitude * -1) - 5763) * -1
}

func (g *Game) startScreenShake(duration, magnitude float64) {
	g.shakeDuration = duration
	g.shakeTimer = duration
	g.shakeMagnitude = magnitude
}

func (g *Game) updateScreenShake(delta float64) {
	if g.shakeTimer <= 0 {
		g.shakeOffsetX = 0
		g.shakeOffsetY = 0
		return
	}
	g.shakeTimer -= delta
	if g.shakeTimer < 0 {
		g.shakeTimer = 0
	}
	progress := g.shakeTimer / g.shakeDuration
	intensity := g.shakeMagnitude * progress
	g.shakeOffsetX = (rand.Float64()*2 - 1) * intensity
	g.shakeOffsetY = (rand.Float64()*2 - 1) * intensity
}

func (g *Game) handleChargePress(key ebiten.Key) {
	if !(rsg >= 3 && !g.launched && !g.gameOver) {
		return
	}
	added := basePowerGain
	if g.lastComboKey != 0 && g.lastComboKey != key && g.comboTimer > 0 {
		g.comboCount++
	} else {
		g.comboCount = 1
	}
	g.lastComboKey = key
	g.comboTimer = comboTimeout
	bonus := float64(g.comboCount-1) * comboBonus
	added += bonus
	g.power += added
	g.totalFuel += added
	if g.power > g.powerMax {
		g.power = g.powerMax
	}
	g.tapCount++
	if g.comboCount > g.maxCombo {
		g.maxCombo = g.comboCount
	}
	playSFX(sfxChargeData)
}

func (g *Game) updateComboTimer(delta float64) {
	if g.comboTimer > 0 {
		g.comboTimer -= delta
		if g.comboTimer <= 0 {
			g.comboTimer = 0
			g.comboCount = 0
			g.lastComboKey = 0
		}
	}
}

func (g *Game) startLaunch() {
	g.launched = true
	g.power_down = false
	g.runDuration = 0
	g.peakSpeed = 0
	g.startScreenShake(0.6, 6)
	g.comboCount = 0
	g.comboTimer = 0
}

func (g *Game) finalizeRun() {
	g.altitude = -5763
	g.speed = 0
	g.launched = false
	g.power_down = false
	g.power = 0
	g.gameOver = true
	g.start_count = false
	duration := g.runDuration
	prep := g.prepDuration
	averageTPS := 0.0
	if prep > 0 {
		averageTPS = float64(g.tapCount) / prep
	}
	g.lastResult = ResultStats{
		Altitude:      g.highscore,
		PeakSpeed:     g.peakSpeed,
		Duration:      duration,
		PrepDuration:  prep,
		TapCount:      g.tapCount,
		MaxCombo:      g.maxCombo,
		AverageTPS:    averageTPS,
		FuelCollected: g.totalFuel,
	}
	g.comboCount = 0
	g.comboTimer = 0
	if g.highscore > g.saved_highscore {
		g.saved_highscore = g.highscore
		if err := g.saveHighscore(); err != nil {
			log.Println("failed to save highscore:", err)
		}
	}
}

func (g *Game) saveHighscore() error {
	data, err := json.Marshal(struct {
		Score float64 `json:"score"`
	}{Score: g.saved_highscore})
	if err != nil {
		return err
	}
	return os.WriteFile(highscoreFile, data, 0o644)
}

func (g *Game) loadHighscore() {
	data, err := os.ReadFile(highscoreFile)
	if err != nil {
		return
	}
	var payload struct {
		Score float64 `json:"score"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		log.Println("failed to parse highscore file:", err)
		return
	}
	g.saved_highscore = payload.Score
}

func (g *Game) playCountdownVoice(number int) {
	if len(voiceSamples) == 0 {
		return
	}
	if g.voicePlayer != nil {
		g.voicePlayer.Close()
	}
	g.voicePlayer = playVoiceClip(number)
}

func (g *Game) Update() error {
	const deltaTime = 1.0 / 60.0

	g.updateScreenShake(deltaTime)
	g.updateComboTimer(deltaTime)

	if g.gameOver {
		if g.JustPressed(ebiten.KeyR) {
			g.restartGame()
		}
		for k := ebiten.Key(0); k <= ebiten.KeyMax; k++ {
			g.prevKeys[k] = ebiten.IsKeyPressed(k)
		}
		return nil
	}

	// Move Clouds
	g.bgoffset -= 30 * deltaTime
	if g.bgoffset <= -float64(screenWidth) {
		g.bgoffset = 0
	}

	// Ready, Set, Go Timer
	if rsg < 3 {
		g.rsg_timer += deltaTime
		if g.rsg_timer >= 1 {
			rsg++
			playSFX(sfxCountData)
			g.rsg_timer = 0
			if rsg == 3 {
				g.playCountdownVoice(count)
			}
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

	if g.JustPressed(ebiten.KeyR) {
		g.restartGame()
	}

	if g.JustPressed(ebiten.KeyZ) {
		g.handleChargePress(ebiten.KeyZ)
	}
	if g.JustPressed(ebiten.KeyX) {
		g.handleChargePress(ebiten.KeyX)
	}

	if rsg >= 3 && !g.launched {
		if !g.start_count {
			playSFX(sfxCountDownData)
			g.start_count = true
			g.prepDuration = 0
			g.tapCount = 0
			g.maxCombo = 0
			g.comboCount = 0
			g.comboTimer = 0
			g.lastComboKey = 0
			g.totalFuel = 0
			g.playCountdownVoice(count)
		}
		g.prepDuration += deltaTime
		if count > 0 {
			g.count_timer += deltaTime
			if g.count_timer >= 1 {
				count--
				g.count_timer = 0
				if count >= 0 {
					g.playCountdownVoice(count)
				}
			}
		} else if !g.launched {
			if g.launchSFXPlayer == nil || !g.launchSFXPlayer.IsPlaying() {
				g.launchSFXPlayer = playSFX(sfxLaunchData)
			}
			g.startLaunch()
		}
	}

	if g.launched && g.power <= 0 {
		if g.launchSFXPlayer != nil {
			g.launchSFXPlayer.Pause()
			g.launchSFXPlayer.Rewind()
			g.launchSFXPlayer = nil
		}
		if !g.power_down {
			playSFX(sfxPowerDownData)
			g.power_down = true
		}
	}

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

	if g.launched {
		g.runDuration += deltaTime
		if g.speed < g.speedMax {
			if g.power > 0 {
				g.speed += 0.6 * deltaTime
				g.power -= 60 * deltaTime
				if g.power < 0 {
					g.power = 0
				}
			} else if g.speed > g.gravity && g.altitude >= -5763 {
				g.speed -= 2.4 * deltaTime
			}
		}
		if g.speed > g.peakSpeed {
			g.peakSpeed = g.speed
		}
		if g.altitude < g.speed && g.altitude >= -5763 {
			g.altitude += g.speed
		}
		if g.altitude < -5763 {
			g.altitude = -5763
		}
		alt := g.currentAltitude()
		if alt > g.highscore {
			g.highscore = alt
		}
		if g.power_down && g.altitude <= -5763 {
			g.finalizeRun()
		}
	}

	for k := ebiten.Key(0); k <= ebiten.KeyMax; k++ {
		g.prevKeys[k] = ebiten.IsKeyPressed(k)
	}

	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	shakeX := g.shakeOffsetX
	shakeY := g.shakeOffsetY
	textOffsetX := int(math.Round(shakeX))
	textOffsetY := int(math.Round(shakeY))

	// Draw Background
	bgOp := &ebiten.DrawImageOptions{}
	bgOp.GeoM.Translate(shakeX, g.altitude+shakeY)
	screen.DrawImage(bg, bgOp)

	// Draw Clouds
	cloudWidth := clouds.Bounds().Dx()

	cloudsOp := &ebiten.DrawImageOptions{}
	cloudsOp.GeoM.Translate(g.bgoffset+shakeX, g.altitude+5740+shakeY)
	screen.DrawImage(clouds, cloudsOp)

	cloudsOp2 := &ebiten.DrawImageOptions{}
	cloudsOp2.GeoM.Translate(g.bgoffset+float64(cloudWidth)+shakeX, g.altitude+5740+shakeY)
	screen.DrawImage(clouds, cloudsOp2)

	// Draw the Highscore Record
	recordOp := &ebiten.DrawImageOptions{}
	recordOp.GeoM.Translate(shakeX, g.altitude+6015-g.saved_highscore+shakeY)
	screen.DrawImage(record, recordOp)
	formatHighscore := fmt.Sprintf("%.0fm", g.saved_highscore)
	boundsHighscore := text.BoundString(myFont, formatHighscore)
	textWidthHighscore := boundsHighscore.Dx()
	xHighscore := (screenWidth - textWidthHighscore) / 2

	customColor := color.RGBA{R: 9, G: 27, B: 162, A: 127}
	text.Draw(screen, formatHighscore, myFont, xHighscore+textOffsetX, int(g.altitude+6080-g.saved_highscore+shakeY), customColor)

	// Draw Particles
	for _, p := range g.particles {
		alpha := uint8(p.Opacity * 255)
		col := color.RGBA{255, 255, 255, alpha}
		drawCircle(screen, p.X+shakeX, p.Y+shakeY, p.Radius, col)
	}

	// Draw the Player
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(195+shakeX, 300+shakeY)
	screen.DrawImage(player, op)

	// Draw Ready, Set, Go
	readyOp := &ebiten.DrawImageOptions{}
	readyOp.GeoM.Translate(float64((screenWidth-303)/2)+shakeX, 130+shakeY)

	i := rsg
	sx, sy := 0+i*303, 4
	sub := ready_set_go.SubImage(image.Rect(sx, sy, sx+303, sy+118)).(*ebiten.Image)
	screen.DrawImage(sub, readyOp)

	// Draw Countdown
	if rsg >= 3 {
		countOp := &ebiten.DrawImageOptions{}
		countOp.GeoM.Translate(float64((screenWidth-159)/2)-5+shakeX, 130+shakeY)

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
		drawTextWithOutline(screen, formatAlt, myFont, x+textOffsetX, 80+textOffsetY, color.White, color.Black)
	} else if g.start_count && !g.launched {
		drawTextWithOutline(screen, "Charge Your Rocket!", myFont, 60+textOffsetX, 80+textOffsetY, color.White, color.Black)

		// Draw Z Button
		zOp := &ebiten.DrawImageOptions{}
		zOp.GeoM.Translate(280+shakeX, 270+shakeY)
		zi := g.z_down
		zsx, zsy := 0+zi*135, 2
		zSub := zbutton.SubImage(image.Rect(zsx, zsy, zsx+135, zsy+135)).(*ebiten.Image)
		screen.DrawImage(zSub, zOp)

		// Draw X Button
		xOp := &ebiten.DrawImageOptions{}
		xOp.GeoM.Translate(310+shakeX, 340+shakeY)
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

		ebitenutil.DrawRect(screen, barX+shakeX, barY+shakeY, barWidth, barHeight, color.RGBA{0, 0, 0, 180})

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
			ebitenutil.DrawRect(screen, barX+2+shakeX, barY+2+shakeY, fillWidth, barHeight-4, color.RGBA{255, 165, 0, 255})

			label := fmt.Sprintf("Fuel %3.0f%%", percent*100)
			bounds := text.BoundString(myFont, label)
			textX := int(barX + (barWidth-float64(bounds.Dx()))/2)
			textY := int(barY) + int(barHeight) - 4
			drawTextWithOutline(screen, label, myFont, textX+textOffsetX, textY+textOffsetY, color.White, color.Black)
		}
	}

	if g.start_count && !g.launched && g.comboCount > 0 {
		percent := g.comboTimer / comboTimeout
		if percent < 0 {
			percent = 0
		}
		if percent > 1 {
			percent = 1
		}
		barWidth := 220.0
		barHeight := 16.0
		barX := (float64(screenWidth) - barWidth) / 2
		barY := 520.0
		ebitenutil.DrawRect(screen, barX-2+shakeX, barY-2+shakeY, barWidth+4, barHeight+4, color.RGBA{0, 0, 0, 180})
		ebitenutil.DrawRect(screen, barX+shakeX, barY+shakeY, barWidth*percent, barHeight, color.RGBA{255, 94, 0, 255})
		comboLabel := fmt.Sprintf("Combo x%d", g.comboCount)
		drawTextWithOutline(screen, comboLabel, myFont, int(barX)+textOffsetX, int(barY)-10+textOffsetY, color.White, color.Black)
		if g.prepDuration > 0 {
			rate := float64(g.tapCount) / g.prepDuration
			rateLabel := fmt.Sprintf("TPS %.1f", rate)
			drawTextWithOutline(screen, rateLabel, myFont, 40+textOffsetX, int(barY)+textOffsetY+12, color.White, color.Black)
		}
	}

	if g.gameOver {
		ebitenutil.DrawRect(screen, 0, 0, float64(screenWidth), float64(screenHeight), color.RGBA{0, 0, 0, 160})
		panelW := 360.0
		panelH := 280.0
		panelX := (float64(screenWidth) - panelW) / 2
		panelY := 150.0
		ebitenutil.DrawRect(screen, panelX, panelY, panelW, panelH, color.RGBA{18, 22, 36, 230})
		titleY := int(panelY) + 48
		drawTextWithOutline(screen, "Flight Results", myFont, int(panelX)+46, titleY, color.White, color.Black)
		instrY := titleY + 36
		drawTextWithOutline(screen, "Press R to relaunch", myFont, int(panelX)+40, instrY, color.White, color.Black)
		stats := []string{
			fmt.Sprintf("Altitude: %.0fm", g.lastResult.Altitude),
			fmt.Sprintf("Best: %.0fm", g.saved_highscore),
			fmt.Sprintf("Flight Time: %.1fs", g.lastResult.Duration),
			fmt.Sprintf("Prep Time: %.1fs", g.lastResult.PrepDuration),
			fmt.Sprintf("Peak Speed: %.1f", g.lastResult.PeakSpeed),
			fmt.Sprintf("Fuel Collected: %.0f", g.lastResult.FuelCollected),
			fmt.Sprintf("Combo Max: x%d", g.lastResult.MaxCombo),
			fmt.Sprintf("Taps: %d (%.1f TPS)", g.lastResult.TapCount, g.lastResult.AverageTPS),
		}
		lineY := instrY + 44
		for _, line := range stats {
			text.Draw(screen, line, myFont, int(panelX)+40, lineY, color.White)
			lineY += 36
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
		powerMax:        900,
		gravity:         -40,
		start_count:     false,
		power_down:      false,
		saved_highscore: 0,
		prevKeys:        make(map[ebiten.Key]bool),
	}
	game.loadHighscore()

	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
