//     ▄▄▄       ▄████▄   ██▓▓█████▄  ▄▄▄▄    ██▀███   ▄▄▄       ██▓ ███▄    █
//    ▒████▄    ▒██▀ ▀█  ▓██▒▒██▀ ██▌▓█████▄ ▓██ ▒ ██▒▒████▄    ▓██▒ ██ ▀█   █
//    ▒██  ▀█▄  ▒▓█    ▄ ▒██▒░██   █▌▒██▒ ▄██▓██ ░▄█ ▒▒██  ▀█▄  ▒██▒▓██  ▀█ ██▒
//    ░██▄▄▄▄██ ▒▓▓▄ ▄██▒░██░░▓█▄   ▌▒██░█▀  ▒██▀▀█▄  ░██▄▄▄▄██ ░██░▓██▒  ▐▌██▒
//     ▓█   ▓██▒▒ ▓███▀ ░░██░░▒████▓ ░▓█  ▀█▓░██▓ ▒██▒ ▓█   ▓██▒░██░▒██░   ▓██░
//     ▒▒   ▓▒█░░ ░▒ ▒  ░░▓   ▒▒▓  ▒ ░▒▓███▀▒░ ▒▓ ░▒▓░ ▒▒   ▓▒█░░▓  ░ ▒░   ▒ ▒
//      ▒   ▒▒ ░  ░  ▒    ▒ ░ ░ ▒  ▒ ▒░▒   ░   ░▒ ░ ▒░  ▒   ▒▒ ░ ▒ ░░ ░░   ░ ▒░
//      ░   ▒   ░         ▒ ░ ░ ░  ░  ░    ░   ░░   ░   ░   ▒    ▒ ░   ░   ░ ░
//          ░  ░░ ░       ░     ░     ░         ░           ░  ░ ░           ░
//              ░             ░            ░

package main

import (
	"fmt"
	"os"
	"os/signal"
	"time"

	"go.bug.st/serial"
	"periph.io/x/conn/v3/physic"
	"periph.io/x/conn/v3/spi"
	"periph.io/x/conn/v3/spi/spireg"
	"periph.io/x/host/v3"
)

// Declaring a variable: note names!
// ... is an indefinite array
var noteNames = [...]string{
	"C", "C#", "D", "D#", "E", "F",
	"F#", "G", "G#", "A", "A#", "B",
}

// Set BPM to midi slave
const internalBpm float32 = 120.0

type Scale struct {
	Name  string
	Steps []int // slice of steps within the scale
}

func scales() []Scale {
	return []Scale{
		{Name: "Major", Steps: []int{0, 2, 4, 5, 7, 9, 11}},
		{Name: "Minor", Steps: []int{0, 2, 3, 5, 7, 8, 10}},
		{Name: "Dorian", Steps: []int{0, 2, 3, 5, 7, 9, 10}},
		{Name: "Phrygian", Steps: []int{0, 1, 3, 5, 7, 8, 10}},
		{Name: "Lydian", Steps: []int{0, 2, 4, 6, 7, 9, 11}},
		{Name: "Mixolydian", Steps: []int{0, 2, 4, 5, 7, 9, 10}},
		{Name: "Locrian", Steps: []int{0, 1, 3, 5, 6, 8, 10}},
	}
}

type Params struct {
	Key        int     // key of the scale (0-11)
	ScaleIdx   int     // index of the scale in the scales slice
	Steps      int     // number of steps in the scale
	Density    float32 // density of the scale (0-1)
	Complexity float32 // complexity of the scale (0-1)
	Swing      float32 // swing of the scale (0-1)
	OctaveSpan int     // number of octaves to span (1-3)
	AccentProb float32 // probability of accenting a note (0-1)
	SlideProb  float32 // probability of sliding to the next note (0-1)
	Gate       float32 // gate length of the notes (0-1)
	Bpm        float32
}

// Pattern sequencer //
type Pattern struct {
	seed [32]uint32 // 32 steps
}

func mix(x, salt uint32) uint32 {
	x += salt * 0x9E3779B9
	x ^= x >> 16
	x *= 0x21F0AAAD
	x ^= x >> 15
	return x
}
func randf(seed, salt uint32) float32 {
	return float32(mix(seed, salt)&0xFFFFFF) / float32(0x1000000)
}

func (p *Pattern) reseed(master uint32) {
	s := master
	for i := 0; i < 32; i++ { // starting at 0, as long as i is less than 32, increment i
		s = mix(s, 0xC0FFEE)
		p.seed[i] = s
	}
}

// Produce a note
type RealizedStep struct {
	Active bool
	Note   int
	Accent bool
	Slide  bool
}

func (p *Pattern) realize(i int, params Params) RealizedStep {
	out := RealizedStep{}
	s := p.seed[i%32]
	if randf(s, 1) > params.Density {
		return out
	}
	out.Active = true
	sc := scales()[params.ScaleIdx].Steps
	degree := 0
	if randf(s, 2) < params.Complexity {
		span := 1 + int(randf(s, 3)*float32(len(sc)-1))
		degree = int(randf(s, 4)*float32(span)) % len(sc)
	}
	root := 36 + params.Key
	out.Note = clampInt(root+sc[degree]+12*octave(s, params), 0, 127)
	out.Accent = randf(s, 7) < params.AccentProb
	out.Slide = randf(s, 8) < params.SlideProb
	return out
}

// Sequencer //
type Sequencer struct {
	port    *MidiPort
	params  Params
	pattern Pattern

	playing      bool // Is your sequencer running? Better go catch it!
	stepIndex    int  // Current step in the pattern
	soundingNote int  // What note is currently online?

	// Live updates, declare channel field
	paramUpdates chan Params

	// Pulse tracking
	pulseInStep  int
	noteOffPulse int
	globalPulse  int

	// Slide
	prevSlide bool

	// Drone
	drone bool

	// Pattern generation
	seedCounter uint32
}

func NewSequencer(port *MidiPort) *Sequencer {
	s := &Sequencer{
		port:         port,
		playing:      true,
		soundingNote: -1,
		paramUpdates: make(chan Params, 1),
	}
	s.pattern.reseed(12345)
	return s
}

// Turn off old note, advance to next step, and play new note
// Deprecated for startStep //
//func (s *Sequencer) advance() {
//	// Turn off old note
//	if s.soundingNote >= 0 {
//		s.port.NoteOff(s.soundingNote, 0)
//		s.soundingNote = -1
//	}
//	// Play new note
//	step := s.pattern.realize(s.stepIndex, s.params)
//	if step.Active {
//		velocity := 100
//		if step.Accent {
//			velocity = 127
//		}
//		s.port.NoteOn(step.Note, velocity, 0)
//		s.soundingNote = step.Note
//	}
//	// Advance to next step
//	s.stepIndex = (s.stepIndex + 1) % s.params.Steps
//}

func (s *Sequencer) startStep() {
	step := s.pattern.realize(s.stepIndex, s.params)
	if step.Active {
		velocity := 100
		if step.Accent {
			velocity = 127
		}

		if s.soundingNote >= 0 && s.prevSlide {
			s.port.NoteOn(step.Note, velocity, 0)
			s.port.NoteOff(s.soundingNote, 0)
		} else {
			if s.soundingNote >= 0 {
				s.port.NoteOff(s.soundingNote, 0)
			}
			s.port.NoteOn(step.Note, velocity, 0)
		}

		s.soundingNote = step.Note
		s.prevSlide = step.Slide

		gatePulses := int(s.params.Gate * 6)
		if gatePulses < 1 {
			gatePulses = 1
		}
		s.noteOffPulse = s.globalPulse + gatePulses
	}

	s.stepIndex = (s.stepIndex + 1) % s.params.Steps
}

func (s *Sequencer) checkNoteOff() {
	if s.soundingNote >= 0 && s.globalPulse >= s.noteOffPulse {
		s.port.NoteOff(s.soundingNote, 0)
		s.soundingNote = -1
	}
}

// Play / Pause / Drone //
func (s *Sequencer) Play() {
	s.playing = true
}

func (s *Sequencer) Pause() {
	s.playing = false
	if s.soundingNote >= 0 {
		s.port.NoteOff(s.soundingNote, 0)
		s.soundingNote = -1
	}
}

func (s *Sequencer) ToggleDrone() {
	s.drone = !s.drone
	if s.drone {
		note := 36 + s.params.Key
		s.port.NoteOn(note, 100, 0)
		s.soundingNote = note
	} else {
		if s.soundingNote >= 0 {
			s.port.NoteOff(s.soundingNote, 0)
			s.soundingNote = -1
		}
	}
}

func (s *Sequencer) Regenerate() {
	s.seedCounter++
	s.pattern.reseed(s.seedCounter)
}

// Clock loop
func (s *Sequencer) runClock() {
	for {
		select {
		case newParams := <-s.paramUpdates:
			s.params = newParams
		default:
		}

		pulseDuration := time.Duration(60000/s.params.Bpm/24) * time.Millisecond

		if s.playing && !s.drone {
			if s.pulseInStep == 0 {
				s.startStep()
			}
			s.checkNoteOff()
			s.globalPulse++
			s.pulseInStep = (s.pulseInStep + 1) % 6
		}
		time.Sleep(pulseDuration)
	}
}

func octave(s uint32, params Params) int {
	if params.OctaveSpan > 1 && randf(s, 5) < params.Complexity*0.6 {
		return 1 + int(randf(s, 6)*float32(params.OctaveSpan-1))
	}
	return 0
}

// Defensive code, are values within expected range?
func clampInt(value, low, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

// Midi output port
type MidiPort struct {
	live bool
	conn serial.Port
}

// Err check, is the hardware online?
func NewMidiPort(device string) *MidiPort {
	m := &MidiPort{live: false}

	mode := &serial.Mode{BaudRate: 31250}
	conn, err := serial.Open(device, mode)
	if err != nil {
		fmt.Printf("[midi] %s unavailiable, printing instead\n", device)
		return m
	}

	m.conn = conn
	m.live = true
	return m
}
func (m *MidiPort) send(data []byte) {
	if !m.live {
		fmt.Printf("MIDI: % X\n", data)
		return
	}
	_, err := m.conn.Write(data)
	if err != nil {
		fmt.Print("[midi] write failed: %v\n", err)
	}
}

// Note functionality
func (m *MidiPort) NoteOn(note, velocity, channel int) {
	msg := []byte{
		byte(0x90 | (channel & 0x0F)),
		byte(note & 0x7F),
		byte(velocity & 0x7F),
	}
	m.send(msg)
}

func (m *MidiPort) NoteOff(note, channel int) {
	msg := []byte{
		byte(0x80 | (channel & 0x0F)),
		byte(note & 0x7F),
		0,
	}
	m.send(msg)
}

// Safely kill notes //
func (m *MidiPort) AllNotesOff(channel int) {
	msg := []byte{
		byte(0xB0 | (channel & 0x0F)),
		123,
		0,
	}
	m.send(msg)
}

type PotBank struct {
	hardware bool
	// storage for simulated pot values
	sim    [10]float32
	smooth [10]float32

	connA spi.Conn
	connB spi.Conn
	portA spi.PortCloser
	portB spi.PortCloser
}

func NewPotBank() *PotBank {
	b := &PotBank{hardware: false}
	b.sim = [10]float32{0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5}

	if _, err := host.Init(); err != nil {
		fmt.Printf("[pots] host init failed, simulating: %v\n", err)
		return b
	}

	portA, err := spireg.Open("/dev/spidev0.0")
	if err != nil {
		fmt.Printf("[pots] chip A unavailable, simulating: %v\n", err)
		return b
	}
	connA, err := portA.Connect(1350*physic.KiloHertz, spi.Mode0, 8)
	if err != nil {
		fmt.Printf("[pots] chip A connect failed, simulating: %v\n", err)
		portA.Close()
		return b
	}

	portB, err := spireg.Open("/dev/spidev0.1")
	if err != nil {
		fmt.Printf("[pots] chip B unavailable, simulating: %v\n", err)
		portA.Close()
		return b
	}
	connB, err := portB.Connect(1350*physic.KiloHertz, spi.Mode0, 8)
	if err != nil {
		fmt.Printf("[pots] chip B connect failed, simulating: %v\n", err)
		portA.Close()
		portB.Close()
		return b
	}

	b.portA, b.connA = portA, connA
	b.portB, b.connB = portB, connB
	b.hardware = true
	return b
}

// Read the pot values, either from hardware or simulated
func (b *PotBank) Read() [10]float32 {
	var out [10]float32
	for i := 0; i < 10; i++ {
		var v float32
		if b.hardware {
			if i < 8 {
				v = b.readChannel(b.connA, i)
			} else {
				v = b.readChannel(b.connB, i-8)
			}
		} else {
			v = b.sim[i]
		}
		b.smooth[i] += 0.25 * (v - b.smooth[i])
		out[i] = b.smooth[i]
	}
	return out
}

func (b *PotBank) readChannel(conn spi.Conn, channel int) float32 {
	cmd := []byte{0x01, byte((8 + channel) << 4), 0x00}
	reply := make([]byte, 3)
	if err := conn.Tx(cmd, reply); err != nil {
		return 0.5
	}
	value := int(reply[1]&0x03)<<8 | int(reply[2])
	return float32(value) / 1023.0
}

// Simulate a pot value, for testing without hardware
func (b *PotBank) Simulate(index int, value float32) {
	if index >= 0 && index < 10 {
		b.sim[index] = value
	}
}
func mapPots(r []float32) Params { // readings, short lived var
	p := Params{}
	// Map the pot readings to Params
	// What key?
	p.Key = int(r[0] * 12.0) // maps to 0-11
	// What scale?
	p.ScaleIdx = clampInt(int(r[1]*float32(len(scales()))), 0, len(scales())-1)
	// How many steps?
	p.Steps = clampInt(1+int(r[2]*32.0), 1, 32) // maps to 1-32
	// How many notes?
	p.Density = r[3] // Already a float32, 0-1
	// How crazy are we going to get?
	p.Complexity = r[4] // Already a float32, 0-1
	// Swing?
	p.Swing = r[5] // Already a float32, 0-1
	// How many octaves?
	p.OctaveSpan = clampInt(1+int(r[6]*6.0), 1, 6) // maps to 1-7
	// How often do we accent?
	p.AccentProb = r[7]
	// How often do we slide?
	p.SlideProb = r[8]
	// What is the
	p.Gate = 0.05 + r[9]*(1.0-0.05) // maps to 0.05-1.0
	// BPM
	p.Bpm = internalBpm
	return p
}
func main() {
	fmt.Println("☢ AcidBrain is online! ☢")

	// Simulate pot readings //
	bank := NewPotBank()
	bank.Simulate(3, 0.75)
	bank.Simulate(2, 0.5)
	bank.Simulate(4, 0.6)
	bank.Simulate(6, 0.25)
	bank.Simulate(8, 0.08)

	// Read the pot values and map them to Params //
	readings := bank.Read()
	params := mapPots(readings[:]) // slice that covers the entire array

	// Build the pattern and the Midi port //
	port := NewMidiPort("/dev/serial0")
	//	pat := Pattern{} // Deprecated
	//	pat.reseed(12345) // Deprecated
	seq := NewSequencer(port)
	seq.params = params

	// Launch clock //
	go seq.runClock()

	// Clean Shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	go func() {
		<-sigChan
		fmt.Println("\nShutting down, the ACIDBRAIN sleeps...")
		port.AllNotesOff(0)
		os.Exit(0)
	}()

	// Pot-reading loop
	go func() {
		for {
			readings := bank.Read()
			latest := mapPots(readings[:])
			seq.paramUpdates <- latest
			time.Sleep(50 * time.Millisecond)
		}
	}()

	// GUI runs on the main goroutine and blocks until the window closes
	runUI()
}
