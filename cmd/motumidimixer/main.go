package main

import (
	"fmt"
	"github.com/rakyll/portmidi"
	"github.com/tgmpje/motu"
	"log"
	"math"
	"os"
	"time"
)

func main() {
	// Run with argument "<Motu ip>:80"
	m := motu.NewMotu(os.Args[1])

	motuCh := make(chan *motu.Event)
	go m.StartListener(motuCh)

	midiIn, midiOut, midiCh := initializeMidiInterface()
	defer midiIn.Close()

	ticker := time.NewTicker(50 * time.Millisecond)

	var desiredVolume float64
	var currentVolume float64

	for {
		select {
		case event := <-midiCh:
			switch event.Status {
			case 144:
				switch event.Data1 {
				case 16:
					switch event.Data2 {
					case 0:
						// Channel 1 Mute button released
					case 127:
						// Channel 1 Mute button pressed
						m.ToggleFaderMute("mix/main/0/matrix", true)
					}
				}
			case 224:
				// Fader 1 moved
				if event.Data1 < 36 {
					// First part is exponential
					desiredVolume = 8.75866 * math.Log(0.000652867*float64(event.Data1))
				} else {
					// Rest is linear
					desiredVolume = (60.0/127.0)*float64(event.Data1) - 48.0
				}
				// Convert db to gain
				desiredVolume = math.Pow(10, desiredVolume/20)
			}
		case event := <-motuCh:
			switch event.Path {
			case "mix/main/0/matrix/mute":
				if event.Value.(float64) == 1 {
					midiOut.WriteShort(144, 16, 127)
				} else {
					midiOut.WriteShort(144, 16, 0)
				}
			case "mix/main/0/matrix/fader":
				fmt.Printf("Main fader=%v (%v dB)\n", event.Value, 20*math.Log10(event.Value.(float64)))
			default:
				fmt.Printf("Ignored Motu event: %v\n", event)
			}
		case <-ticker.C:
			if currentVolume != desiredVolume {
				if err := m.SetFaderPosition("mix/main/0/matrix", desiredVolume); err != nil {
					fmt.Printf("error setting fader position: %v\n", err)
				} else {
					currentVolume = desiredVolume
				}
			}
		}
	}
}

func initializeMidiInterface() (*portmidi.Stream, *portmidi.Stream, <-chan portmidi.Event) {
	portmidi.Initialize()
	fmt.Printf("Number of MIDI devices: %v\n", portmidi.CountDevices())
	for i := 0; i < portmidi.CountDevices(); i++ {
		info := portmidi.Info(portmidi.DeviceID(i))
		fmt.Printf("Device %v: %v [input: %v] [output: %v]\n", i, info.Name, info.IsInputAvailable, info.IsOutputAvailable)
	}

	// KORG nanoKONTROL2 input connected to device 0
	midiIn, err := portmidi.NewInputStream(0, 1024)
	if err != nil {
		log.Fatal(err)
	}

	// KORG nanoKONTROL2 output (for changing leds) connected to device 1
	midiOut, err := portmidi.NewOutputStream(1, 1024, 0)
	if err != nil {
		log.Fatal(err)
	}

	midiCh := midiIn.Listen()

	return midiIn, midiOut, midiCh
}
