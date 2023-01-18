package main

import (
	"log"
	"math"
	"fmt"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/randr"
	"github.com/jezek/xgb/xproto"
	"gitlab.com/lehn/edid"
)

func getModeInfo(resources *randr.GetScreenResourcesReply, mode randr.Mode) *randr.ModeInfo {
	for _, modeInfo := range resources.Modes {
		if modeInfo.Id == uint32(mode) {
			return &modeInfo
		}
	}

	return nil
}

func findMode(resources *randr.GetScreenResourcesReply, name string, refresh float64) *randr.ModeInfo {
	var best *randr.ModeInfo
	var bestDist float64

	for _, modeInfo := range resources.Modes {
		dist := 0.0
		if refresh != 0.0 {
			dist = math.Abs(refreshRate(&modeInfo) - refresh)
		}
		if best == nil || dist < bestDist {
			best = &modeInfo
			bestDist = dist
		}
	}

	return best
}

func modeName(modeInfo *randr.ModeInfo) string {
	return fmt.Sprintf("%dx%d", modeInfo.Width, modeInfo.Height)
}

func refreshRate(modeInfo *randr.ModeInfo) float64 {
	if modeInfo.Htotal == 0 || modeInfo.Vtotal == 0 {
		return 0
	}

	return float64(modeInfo.DotClock) / (float64(modeInfo.Htotal) * float64(modeInfo.Vtotal))
}

func bestMode(screen *xproto.ScreenInfo, resources *randr.GetScreenResourcesReply, info *randr.GetOutputInfoReply) randr.Mode {
	var best randr.Mode
	var dist int
	bestDist := 0

	for m, mode := range info.Modes {
		modeInfo := getModeInfo(resources, mode)
		if m < int(info.NumPreferred) {
			dist = 0
		} else if info.MmHeight > 0 {
			dist = 1000*int(screen.HeightInPixels)/int(screen.HeightInMillimeters) -
				1000*int(modeInfo.Height)/int(info.MmHeight)
		} else {
			dist = int(screen.HeightInPixels) - int(modeInfo.Height)
		}

		if dist < 0 {
			dist = -dist
		}
		if best == 0 || dist < bestDist {
			best = mode
			bestDist = dist
		}
	}

	return best
}

func pickAvailableCrtc(X *xgb.Conn, resources *randr.GetScreenResourcesReply, output randr.Output) (randr.Crtc, error) {
	for _, crtc := range resources.Crtcs {
		crtcInfo, err := randr.GetCrtcInfo(X, crtc, 0).Reply()
		if err != nil {
			return 0, err
		}

		for _, possibleOutput := range crtcInfo.Possible {
			if possibleOutput == output {
				used := false

				// Check if used anywhere else
				for _, otherOutput := range resources.Outputs {
					if otherOutput == output {
						continue
					}
					info, err := randr.GetOutputInfo(X, otherOutput, 0).Reply()
					if err != nil {
						return 0, err
					}
					if info.Crtc == crtc {
						used = true
						break
					}
				}

				if !used {
					return crtc, nil
				}
			}
		}
	}

	log.Printf("no CRTC available\n")
	return 0, fmt.Errorf("no CRTC available")
}
func getMonitorIdentifier(X *xgb.Conn, output randr.Output) string {
	at, err := xproto.InternAtom(X, false, 4, "EDID").Reply()
	if err != nil {
		log.Printf("InternAtom %v\n", err)
		return ""
	}

	prop, err := randr.GetOutputProperty(X, output, at.Atom, xproto.GetPropertyTypeAny, 0, 128, false, false).Reply()
	if err != nil {
		log.Printf("GetOutputProperty %v\n", err)
		return ""
	}

	if len(prop.Data) < 128 {
		return ""
	}

	e, err := edid.New(prop.Data)
	if err != nil {
		return ""
	}

	return fmt.Sprintf("%s-%d-%d", string(e.PNPID[:]), e.Model, e.Serial)
}

type Output struct {
	id        randr.Output
	info      *randr.GetOutputInfoReply
	monitorId string
	x, y      int
	crtc      randr.Crtc
	modeInfo  *randr.ModeInfo
	crtcInfo  *randr.GetCrtcInfoReply
	disable   bool
}

func getOutputs(X *xgb.Conn, resources *randr.GetScreenResourcesReply) ([]*Output, error) {
	outputs := make([]*Output, 0, resources.NumOutputs)

	for _, outputId := range resources.Outputs {
		info, err := randr.GetOutputInfo(X, outputId, 0).Reply()
		if err != nil {
			log.Printf("GetOutputInfo %v\n", err)
			return nil, err
		}

		monitorId := getMonitorIdentifier(X, outputId)

		outputs = append(outputs, &Output{id: outputId, info: info, monitorId: monitorId})
	}

	return outputs, nil
}

type Rect struct {
	left, top, right, bottom int
}

func (r *Rect) update(x, y, width, height int) {
	if r.left == 0 && r.right == 0 && r.top == 0 && r.bottom == 0 {
		r.left = x
		r.right = x + width
		r.top = y
		r.bottom = y + height
	} else {
		if x < r.left {
			r.left = x
		}
		if y < r.top {
			r.top = y
		}
		if x + width > r.right {
			r.right = x + width
		}
		if y + height > r.bottom {
			r.bottom = y + height
		}
	}
}

func shouldDisableOnLidClose(conf *Config, setupName string, output *Output) bool {
	if setupName == "" {
		return false
	}
	oc := conf.getOutputConfig(setupName, output.monitorId)
	return oc != nil && oc.DisableOnLidClose
}

func (s *State) reconfigureOutputs() error {
	err := xproto.GrabServerChecked(s.X).Check()
	if err != nil {
		log.Printf("GrabServer %v\n", err)
		return err
	}
	var change bool

	defer func () {
		xproto.UngrabServerChecked(s.X).Check()
		if (change) {
			s.conf.executeBackgroundCommand()
		}
	}()

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

	outputs, err := getOutputs(s.X, resources)
	if err != nil {
		return err
	}

	lidClosed, err := s.isLidClosed()
	if err != nil {
		return err
	}

	var resetPrimary bool
	var newPrimary *Output

	setupName := s.conf.findBestSetup(outputs)
	if setupName != "" {
		log.Printf("Found matching setup from config: %s\n", setupName)
	}

	// Check monitors that need disabling
	for _, output := range outputs {
		if output.info.Crtc != 0 {
			if output.info.Connection == randr.ConnectionDisconnected {
				log.Printf("Turning off: %s\n", string(output.info.Name))
				output.disable = true
			} else if lidClosed && shouldDisableOnLidClose(s.conf, setupName, output) {
				log.Printf("Disabling %s because lid is closed\n", string(output.info.Name))
				output.disable = true
			}
		}
	}

	// Turn off monitors
	for _, output := range outputs {
		if output.disable {
			_, err = randr.SetCrtcConfig(s.X, output.info.Crtc, 0, 0, 0, 0, 0,
				randr.RotationRotate0, []randr.Output{}).Reply()
			if err != nil {
				log.Printf("SetCrtcConfig %v\n", err)
				return err
			}
		}
	}

	var display Rect

	// Handle unchanged output
	for _, output := range outputs {
		if !output.disable && output.info.Crtc != 0 {
			output.crtc = output.info.Crtc
			output.crtcInfo, err = randr.GetCrtcInfo(s.X, output.crtc, 0).Reply()
			if err != nil {
				log.Printf("GetCrtcInfo %v\n", err)
				return err
			}
			output.modeInfo = getModeInfo(resources, output.crtcInfo.Mode)

			// Only reposition if we're in a particular setup
			oc := s.conf.getOutputConfig(setupName, output.monitorId)
			if oc != nil {
				output.x = oc.Position.X
				output.y = oc.Position.Y
				if oc.Primary && output.id != primary.Output {
					resetPrimary = true
					newPrimary = output
				}
			} else {
				output.x = int(output.crtcInfo.X)
				output.y = int(output.crtcInfo.Y)
			}

			display.update(output.x, output.y, int(output.modeInfo.Width), int(output.modeInfo.Height))
		}
	}

	// Configure recently connected monitors
	for _, output := range outputs {
		if output.info.Connection != randr.ConnectionDisconnected && output.info.Crtc == 0 {
			if lidClosed && shouldDisableOnLidClose(s.conf, setupName, output) {
				continue
			}


			log.Printf("Turning on: %s\n", string(output.info.Name))
			log.Printf("EDID: %s\n", output.monitorId)

			output.crtc, err = pickAvailableCrtc(s.X, resources, output.id)
			if err != nil {
				return err
			}

			log.Printf("Picking CRTC=%x\n", uint32(output.crtc))
			output.crtcInfo, err = randr.GetCrtcInfo(s.X, output.crtc, 0).Reply()
			if err != nil {
				log.Printf("GetCrtcInfo %v\n", err)
				return err
			}

			mode := bestMode(s.screen, resources, output.info)
			log.Printf("Using mode=%x\n", uint32(mode))
			output.modeInfo = getModeInfo(resources, mode)

			oc := s.conf.getOutputConfig(setupName, output.monitorId)
			if oc != nil {
				output.x = oc.Position.X
				output.y = oc.Position.Y
				if oc.Primary && output.id != primary.Output {
					resetPrimary = true
					newPrimary = output
				}
			} else {
				switch s.conf.DefaultExtendDirection {
				case "", "right":
					output.x = display.right
					output.y = 0
				case "left":
					output.x = -int(output.modeInfo.Width)
					output.y = 0
				case "bottom":
					output.x = 0
					output.y = display.bottom
				case "top":
					output.x = 0
					output.y = -int(output.modeInfo.Height)
				default:
					return fmt.Errorf("invalid default extend direction: %s", s.conf.DefaultExtendDirection)
				}
			}

			display.update(output.x, output.y, int(output.modeInfo.Width), int(output.modeInfo.Height))
		}
	}

	// Normalize output positions
	for _, output := range outputs {
		output.x -= display.left
		output.y -= display.top
	}
	display.right -= display.left
	display.bottom -= display.top
	display.left = 0
	display.top = 0
	log.Printf("Display rect: left %d right %d top %d bottom %d\n", display.left, display.right, display.top, display.bottom)

	// Apply
	for _, output := range outputs {
		if output.modeInfo != nil && output.crtcInfo != nil &&
			(output.crtcInfo.Mode != randr.Mode(output.modeInfo.Id) ||
				output.crtcInfo.X != int16(output.x) ||
				output.crtcInfo.Y != int16(output.y)) {
			log.Printf("Reconfiguring CRTC=%x:\n", uint32(output.crtc))
			log.Printf("  mode %x -> %x\n", output.crtcInfo.Mode, output.modeInfo.Id)
			log.Printf("  position %d+%d -> %d+%d\n", output.crtcInfo.X, output.crtcInfo.Y,
				output.x, output.y)

			_, err = randr.SetCrtcConfig(s.X, output.crtc, 0, 0, int16(output.x), int16(output.y), randr.Mode(output.modeInfo.Id),
				randr.RotationRotate0, []randr.Output{output.id}).Reply()
			if err != nil {
				log.Printf("SetCrtcConfig %v\n", err)
				return err
			}
			change = true
		}
	}

	if uint16(display.right) != s.screen.WidthInPixels ||
		uint16(display.bottom) != s.screen.HeightInPixels {
		dpi := (25.4 * float64(s.screen.HeightInPixels)) / float64(s.screen.HeightInMillimeters)
		screenWidthMM := uint32((25.4 * float64(display.right)) / dpi)
		screenHeightMM := uint32((25.4 * float64(display.bottom)) / dpi)

		log.Printf("Setting screen size %dx%d (%dx%dmm)\n", display.right, display.bottom, screenWidthMM, screenHeightMM)
		err := randr.SetScreenSizeChecked(s.X, s.screen.Root, uint16(display.right), uint16(display.bottom), screenWidthMM, screenHeightMM).Check()
		if err != nil {
			log.Printf("SetScreenSize %v\n", err)
			return err
		}
		s.screen.WidthInPixels = uint16(display.right)
		s.screen.HeightInPixels = uint16(display.bottom)
		s.screen.WidthInMillimeters = uint16(screenWidthMM)
		s.screen.HeightInMillimeters = uint16(screenHeightMM)

		change = true
	}

	if resetPrimary {
		if newPrimary == nil {
			log.Printf("Electing new primary output\n")

			// Need something more clever ?
			for _, output := range outputs {
				if output.info.Connection != randr.ConnectionDisconnected {
					newPrimary = output
					break
				}
			}
			if newPrimary == nil {
				return fmt.Errorf("no candidate for primary output")
			}
		}

		log.Printf("Setting primary output: %s\n", string(newPrimary.info.Name))
		err := randr.SetOutputPrimaryChecked(s.X, s.screen.Root, newPrimary.id).Check()
		if err != nil {
			log.Printf("SetOutputPrimary %v\n", err)
			return err
		}
		change = true
	}

	return nil
}

