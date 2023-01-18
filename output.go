package main

import (
	"fmt"
	"log"

	"github.com/jezek/xgb/randr"
)

func (s *State) readOutputChangeEvents() (chan randr.OutputChange, error) {
	err := randr.SelectInputChecked(s.X, s.screen.Root, randr.NotifyMaskOutputChange).Check()
	if err != nil {
		return nil, err
	}

	events := make(chan randr.OutputChange, 1)

	go func() {
		for {
			ev, xerr := s.X.WaitForEvent()
			if xerr != nil {
				log.Fatalf("WaitForEvent: %s\n", xerr.Error())
			}
			notifyEvent, ok := ev.(randr.NotifyEvent)
			if !ok {
				continue
			}
			events <- notifyEvent.U.Oc
		}
	}()

	return events, nil
}

func (s *State) showOutputs() error {
	resources, err := randr.GetScreenResources(s.X, s.screen.Root).Reply()
	if err != nil {
		log.Printf("GetScreenResources %v\n", err)
		return err
	}

	primary, err := randr.GetOutputPrimary(s.X, s.screen.Root).Reply()
	if err != nil {
		log.Printf("GetOutputPrimary %v\n", err)
		return err
	}

	fmt.Printf("Outputs:\n")

	for _, output := range resources.Outputs {
		info, err := randr.GetOutputInfo(s.X, output, 0).Reply()
		if err != nil {
			log.Printf("GetOutputInfo %v\n", err)
			return err
		}

		fmt.Printf("  [%x] %s: CRTC=%x", output, string(info.Name), info.Crtc)
		if info.Connection != randr.ConnectionDisconnected {
			fmt.Printf(" connected")
		} else {
			fmt.Printf(" disconnected")
		}
		if info.Connection != randr.ConnectionDisconnected {
			mode := bestMode(s.screen, resources, info)
			modeInfo := getModeInfo(resources, mode)
			fmt.Printf(", best mode: %dx%d (%x)", modeInfo.Width, modeInfo.Height, mode)
		}
		if info.Crtc != 0 {
			crtcInfo, err := randr.GetCrtcInfo(s.X, info.Crtc, 0).Reply()
			if err != nil {
				log.Printf("GetCrtcInfo %v\n", err)
				return err
			}
			modeInfo := getModeInfo(resources, crtcInfo.Mode)
			fmt.Printf(", current mode: %dx%d (%x)", modeInfo.Width, modeInfo.Height, crtcInfo.Mode)
			fmt.Printf(", position: %d+%d", crtcInfo.X, crtcInfo.Y)
		}
		ident := getMonitorIdentifier(s.X, output)
		if ident != "" {
			fmt.Printf(", Monitor=%s", ident)
		}
		if output == primary.Output {
			fmt.Printf(", primary")
		}

		fmt.Printf("\n")
	}

	return nil
}


func (s *State) handleOutputChange(change *randr.OutputChange) error {
	info, err := randr.GetOutputInfo(s.X, change.Output, 0).Reply()
	if err != nil {
		log.Printf("GetOutputInfo %v\n", err)
		return err
	}
	is_connected := ((info.Connection == randr.ConnectionConnected) ||
		(info.Connection == randr.ConnectionUnknown))

	log.Printf("output change: %s connected=%v\n", string(info.Name), is_connected)

	return s.reconfigureOutputs()
}
