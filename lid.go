package main

import (
	"log"

	"github.com/godbus/dbus/v5"
)

func (s *State) handleLidClose(closed bool) error {
	if closed {
		log.Printf("New lid state: closed\n")
	} else {
		log.Printf("New lid state: opened\n")
	}

	return s.reconfigureOutputs()
}

func (s *State) isLidClosed() (bool, error) {
	upower := s.bus.Object("org.freedesktop.UPower", "/org/freedesktop/UPower")
	v, err := upower.GetProperty("org.freedesktop.UPower.LidIsClosed")
	if err != nil {
		return false, err
	}
	return v.Value().(bool), nil
}

func (s *State) readLidCloseEvents() (chan bool, error) {
	err := s.bus.AddMatchSignal(
		dbus.WithMatchObjectPath("/org/freedesktop/UPower"),
	)
	if err != nil {
		return nil, err
	}

	events := make(chan bool, 1)

	sigChan := make(chan *dbus.Signal, 10)
	s.bus.Signal(sigChan)

	go func() {
		for v := range sigChan {
			switch v.Name {
			case "org.freedesktop.DBus.Properties.PropertiesChanged":
				properties := v.Body[1].(map[string]dbus.Variant)
				if v, ok := properties["LidIsClosed"]; ok {
					events <- v.Value().(bool)
				}
			}
		}
	}()

	return events, nil
}
