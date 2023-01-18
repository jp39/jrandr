package main

import (
	"log"
	"os"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/jezek/xgb"
	"github.com/jezek/xgb/randr"
	"github.com/jezek/xgb/xproto"
)


type State struct {
	X *xgb.Conn
	bus *dbus.Conn
	screen *xproto.ScreenInfo
	conf *Config
}

func NewState() (*State, error) {
	X, err := xgb.NewConn()
	if err != nil {
		log.Printf("xgb.NewConn: %v", err)
		return nil, err
	}

	err = randr.Init(X)
	if err != nil {
		log.Printf("randr.Init: %v", err)
		X.Close()
		return nil, err
	}

	screen := xproto.Setup(X).DefaultScreen(X)

	bus, err := dbus.SystemBus()
	if err != nil {
		log.Printf("dbus.SystemBus: %v", err)
		X.Close()
		return nil, err
	}

	return &State{X, bus, screen, nil}, nil
}

func (s *State) Cleanup() {
	s.bus.Close()
	s.X.Close()
}

func main() {
	state, err := NewState()
	if err != nil {
		log.Fatalf("NewState failed\n")
	}
	defer state.Cleanup()

	if len(os.Args) >= 2 {
		state.conf, err = parseConfigFile(os.Args[1])
		if err != nil {
			log.Fatalf("parseConfigFile failed: %v\n", err)
		}
	} else {
		state.showOutputs()
		os.Exit(0)
	}

	outputEventChannel, err := state.readOutputChangeEvents()
	if err != nil {
		log.Fatalf("readOutputChangeEvents: %v", err)
	}

	lidEventChannel, err := state.readLidCloseEvents()
	if err != nil {
		log.Fatalf("readLidCloseEvents: %v", err)
	}

	wait, err := time.ParseDuration(state.conf.Wait)
	if err != nil {
		log.Fatalf("Failed to parse duration %s: %v", state.conf.Wait, err)
	}
	log.Printf("Waiting %d seconds before starting\n", int(wait.Seconds()))
	time.Sleep(wait)

	err = state.reconfigureOutputs()
	if err != nil {
		log.Fatalf("reconfigureOutputs: %v", err)
	}

	log.Printf("Processing events...\n")

	for {
		select {
		case change := <-outputEventChannel:
			state.handleOutputChange(&change)
		case lidClosed := <-lidEventChannel:
			state.handleLidClose(lidClosed)
		}
	}
}
