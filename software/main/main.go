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
}

// Turn off old note, advance to next step, and play new note
func (s *Sequencer) advance() {
	// Turn off old note
	if s.soundingNote >= 0 {
		s.port.NoteOff(s.soundingNote, 0)
		s.soundingNote = -1
	}
	// Play new note
	step := s.pattern.realize(s.stepIndex, s.params)
	if step.Active {
		velocity := 100
		if step.Accent {
			velocity = 127
		}
		s.port.NoteOn(step.Note, velocity, 0)
		s.soundingNote = step.Note
	}
	// Advance to next step
	s.stepIndex = (s.stepIndex + 1) % s.params.Steps
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
}

func (m *MidiPort) send(data []byte) {
	if !m.live {
		fmt.Printf("MIDI: % X\n", data)
		return
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

type PotBank struct {
	hardware bool
	// storage for simulated pot values
	sim [10]float32
}

// Read the pot values, either from hardware or simulated
func (b *PotBank) Read() [10]float32 {
	return b.sim
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
	bank := PotBank{}
	bank.Simulate(3, 0.75)
	bank.Simulate(2, 0.5)
	bank.Simulate(4, 0.6)
	bank.Simulate(6, 0.25)

	// Read the pot values and map them to Params //
	readings := bank.Read()
	params := mapPots(readings[:]) // slice that covers the entire array

	// Build the pattern and the Midi port //
	port := MidiPort{}
	pat := Pattern{}
	pat.reseed(12345)

	// Play the pattern //
	for i := 0; i < params.Steps; i++ {
		step := pat.realize(i, params)
		if step.Active {
			port.NoteOn(step.Note, 100, 0)
			port.NoteOff(step.Note, 0)
		}
	}
	// Debug output //
	// port.NoteOn(45, 100, 0)
	// port.NoteOff(45, 0)
	// fmt.Printf("step %2d: %+v\n", i, step)
	// fmt.Printf("%+v\n", params)
}
