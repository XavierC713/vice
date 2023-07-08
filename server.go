// server.go
// Copyright(c) 2023 Matt Pharr, licensed under the GNU Public License, Version 3.
// SPDX: GPL-3.0-only

package main

import (
	"encoding/gob"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

func init() {
	gob.Register(&FlyHeading{})
	gob.Register(&FlyRoute{})
	gob.Register(&FlyRacetrackPT{})
	gob.Register(&FlyStandard45PT{})

	gob.Register(&MaintainSpeed{})
	gob.Register(&FinalApproachSpeed{})

	gob.Register(&MaintainAltitude{})
	gob.Register(&FlyRacetrackPT{})

	gob.Register(&SpeedAfterAltitude{})
	gob.Register(&AltitudeAfterSpeed{})
	gob.Register(&ApproachSpeedAt5DME{})
	gob.Register(&ClimbOnceAirborne{})
	gob.Register(&TurnToInterceptLocalizer{})
	gob.Register(&HoldLocalizerAfterIntercept{})
	gob.Register(&GoAround{})
}

var (
	ErrNoNamedSim                = errors.New("No Sim with that name")
	ErrNoSimForControllerToken   = errors.New("No Sim running for controller token")
	ErrControllerAlreadySignedIn = errors.New("controller with that callsign already signed in")
	ErrInvalidControllerToken    = errors.New("invalid controller token")
)

type SimServer struct {
	name    string
	client  *rpc.Client
	configs map[string]*SimConfiguration
}

///////////////////////////////////////////////////////////////////////////

type SimProxy struct {
	ControllerToken string
	Client          *rpc.Client
}

type AircraftSpecifier struct {
	ControllerToken string
	Callsign        string
}

func (s *SimProxy) TogglePause() *rpc.Call {
	return s.Client.Go("Sim.TogglePause", s.ControllerToken, nil, nil)
}

func (s *SimProxy) SignOff(_, _ *struct{}) error {
	return s.Client.Call("Sim.SignOff", s.ControllerToken, nil)
}

func (s *SimProxy) ChangeControlPosition(callsign string, keepTracks bool) error {
	return s.Client.Call("Sim.ChangeControlPosition",
		&ChangeControlPositionArgs{
			ControllerToken: s.ControllerToken,
			Callsign:        callsign,
			KeepTracks:      keepTracks,
		}, nil)
}

func (s *SimProxy) GetSerializeSim() (*Sim, error) {
	var sim Sim
	err := s.Client.Call("SimManager.GetSerializeSim", s.ControllerToken, &sim)
	return &sim, err
}

func (s *SimProxy) GetWorldUpdate(wu *SimWorldUpdate) *rpc.Call {
	return s.Client.Go("Sim.GetWorldUpdate", s.ControllerToken, wu, nil)
}

func (s *SimProxy) SetSimRate(r float32) *rpc.Call {
	return s.Client.Go("Sim.SetSimRate",
		&SetSimRateArgs{
			ControllerToken: s.ControllerToken,
			Rate:            r,
		}, nil, nil)
}

func (s *SimProxy) TakeOrReturnLaunchControl() *rpc.Call {
	return s.Client.Go("Sim.TakeOrReturnLaunchControl", s.ControllerToken, nil, nil)
}

func (s *SimProxy) SetScratchpad(callsign string, scratchpad string) *rpc.Call {
	return s.Client.Go("Sim.SetScratchpad", &SetScratchpadArgs{
		ControllerToken: s.ControllerToken,
		Callsign:        callsign,
		Scratchpad:      scratchpad,
	}, nil, nil)
}

func (s *SimProxy) InitiateTrack(callsign string) *rpc.Call {
	return s.Client.Go("Sim.InitiateTrack", &InitiateTrackArgs{
		ControllerToken: s.ControllerToken,
		Callsign:        callsign,
	}, nil, nil)
}

func (s *SimProxy) DropTrack(callsign string) *rpc.Call {
	return s.Client.Go("Sim.DropTrack", &DropTrackArgs{
		ControllerToken: s.ControllerToken,
		Callsign:        callsign,
	}, nil, nil)
}

func (s *SimProxy) HandoffTrack(callsign string, controller string) *rpc.Call {
	return s.Client.Go("Sim.HandoffTrack", &HandoffArgs{
		ControllerToken: s.ControllerToken,
		Callsign:        callsign,
		Controller:      controller,
	}, nil, nil)
}

func (s *SimProxy) HandoffControl(callsign string) *rpc.Call {
	return s.Client.Go("Sim.HandoffControl", &HandoffArgs{
		ControllerToken: s.ControllerToken,
		Callsign:        callsign,
	}, nil, nil)
}

func (s *SimProxy) AcceptHandoff(callsign string) *rpc.Call {
	return s.Client.Go("Sim.AcceptHandoff", &AcceptHandoffArgs{
		ControllerToken: s.ControllerToken,
		Callsign:        callsign,
	}, nil, nil)
}

func (s *SimProxy) RejectHandoff(callsign string) *rpc.Call {
	return s.Client.Go("Sim.RejectHandoff", &RejectHandoffArgs{
		ControllerToken: s.ControllerToken,
		Callsign:        callsign,
	}, nil, nil)
}

func (s *SimProxy) CancelHandoff(callsign string) *rpc.Call {
	return s.Client.Go("Sim.CancelHandoff", &CancelHandoffArgs{
		ControllerToken: s.ControllerToken,
		Callsign:        callsign,
	}, nil, nil)
}

func (s *SimProxy) PointOut(callsign string, controller string) *rpc.Call {
	return s.Client.Go("Sim.PointOut", &PointOutArgs{
		ControllerToken: s.ControllerToken,
		Callsign:        callsign,
		Controller:      controller,
	}, nil, nil)
}

func (s *SimProxy) AssignAltitude(callsign string, alt int) *rpc.Call {
	return s.Client.Go("Sim.SetAltitude", &AssignAltitudeArgs{
		ControllerToken: s.ControllerToken,
		Callsign:        callsign,
		Altitude:        alt,
	}, nil, nil)
}

func (s *SimProxy) SetTemporaryAltitude(callsign string, alt int) *rpc.Call {
	return s.Client.Go("Sim.SetTemporaryAltitude", &AssignAltitudeArgs{
		ControllerToken: s.ControllerToken,
		Callsign:        callsign,
		Altitude:        alt,
	}, nil, nil)
}

func (s *SimProxy) GoAround(callsign string) *rpc.Call {
	return s.Client.Go("Sim.GoAround", &GoAroundArgs{
		ControllerToken: s.ControllerToken,
		Callsign:        callsign,
	}, nil, nil)
}

func (s *SimProxy) DeleteAircraft(callsign string) *rpc.Call {
	return s.Client.Go("Sim.DeleteAircraft", &DeleteAircraftArgs{
		ControllerToken: s.ControllerToken,
		Callsign:        callsign,
	}, nil, nil)
}

type AircraftCommandsArgs struct {
	ControllerToken string
	Callsign        string
	Commands        string
}

func (s *SimProxy) RunAircraftCommands(callsign string, cmds string) *rpc.Call {
	return s.Client.Go("Sim.RunAircraftCommands", &AircraftCommandsArgs{
		ControllerToken: s.ControllerToken,
		Callsign:        callsign,
		Commands:        cmds,
	}, nil, nil)
}

func (s *SimProxy) LaunchAircraft(ac Aircraft) *rpc.Call {
	return s.Client.Go("Sim.LaunchAircraft", &LaunchAircraftArgs{
		ControllerToken: s.ControllerToken,
		Aircraft:        ac,
	}, nil, nil)
}

///////////////////////////////////////////////////////////////////////////
// SimManager

type SimManager struct {
	scenarioGroups           map[string]*ScenarioGroup
	configs                  map[string]*SimConfiguration
	activeSims               map[string]*Sim
	controllerTokenToSim     map[string]*Sim
	mu                       sync.Mutex
	sentBytes, receivedBytes int64
	startTime                time.Time
	lastBandwidthLog         time.Time
}

func NewSimManager(scenarioGroups map[string]*ScenarioGroup,
	simConfigurations map[string]*SimConfiguration) *SimManager {
	return &SimManager{
		scenarioGroups:       scenarioGroups,
		configs:              simConfigurations,
		activeSims:           make(map[string]*Sim),
		controllerTokenToSim: make(map[string]*Sim),
		startTime:            time.Now(),
	}
}

type NewSimResult struct {
	World           *World
	ControllerToken string
}

func (sm *SimManager) New(config *NewSimConfiguration, result *NewSimResult) error {
	sm.mu.Lock()

	if config.NewSimType == NewSimCreateLocal || config.NewSimType == NewSimCreateRemote {
		sim := NewSim(*config, sm.scenarioGroups)
		sm.mu.Unlock()
		sim.prespawn()

		return sm.Add(sim, result)
	} else {
		sim, ok := sm.activeSims[config.SelectedRemoteSim]
		if !ok {
			return ErrNoNamedSim
		}
		if _, ok := sim.World.Controllers[config.SelectedRemoteSimPosition]; ok {
			return ErrNoController
		}

		sm.mu.Unlock()
		world, token, err := sim.SignOn(config.SelectedRemoteSimPosition)
		if err != nil {
			return err
		}

		sm.mu.Lock()
		sm.controllerTokenToSim[token] = sim
		sm.mu.Unlock()

		*result = NewSimResult{
			World:           world,
			ControllerToken: token,
		}
		return nil
	}
}

func (sm *SimManager) Add(sim *Sim, result *NewSimResult) error {
	sim.Activate()

	sm.mu.Lock()

	// Empty sim name is just a local sim, so no problem with replacing it...
	if _, ok := sm.activeSims[sim.Name]; ok && sim.Name != "" {
		return errors.New(sim.Name + ": a sim with that name already exists")
	}

	lg.Printf("%s: starting new sim", sim.Name)
	sm.activeSims[sim.Name] = sim

	sm.mu.Unlock()

	world, token, err := sim.SignOn(sim.World.PrimaryController)
	if err != nil {
		return err
	}

	sm.mu.Lock()
	sm.controllerTokenToSim[token] = sim
	sm.mu.Unlock()

	go func() {
		for {
			sim.Update()
			time.Sleep(10 * time.Millisecond)
		}
	}()

	*result = NewSimResult{
		World:           world,
		ControllerToken: token,
	}

	return nil
}

func (sm *SimManager) GetSimConfigurations(_ int, result *map[string]*SimConfiguration) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	*result = sm.configs
	return nil
}

func (sm *SimManager) GetRunningSims(_ int, result *map[string]*RemoteSim) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	running := make(map[string]*RemoteSim)
	for name, s := range sm.activeSims {
		s.mu.Lock()
		rs := &RemoteSim{
			GroupName:          s.ScenarioGroup,
			ScenarioName:       s.Scenario,
			AvailablePositions: make(map[string]struct{}),
		}

		// Figure out which positions are available; start with all of the possible ones,
		// then delete those that are active
		rs.AvailablePositions[s.World.PrimaryController] = struct{}{}
		for callsign := range s.World.MultiControllers {
			rs.AvailablePositions[callsign] = struct{}{}
		}
		for _, ctrl := range s.controllers {
			delete(rs.AvailablePositions, ctrl.Callsign)
		}
		s.mu.Unlock()

		running[name] = rs
	}

	*result = running
	return nil
}

func (sm *SimManager) GetSerializeSim(token string, s *Sim) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.controllerTokenToSim == nil {
		return ErrNoSimForControllerToken
	}
	sim, ok := sm.controllerTokenToSim[token]
	if !ok {
		return ErrNoSimForControllerToken
	}
	*s = *sim
	return nil
}

func (sm *SimManager) ControllerTokenToSim(token string) (*Sim, bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if time.Since(sm.lastBandwidthLog) > 15*time.Second {
		sm.lastBandwidthLog = time.Now()
		lg.Printf("Total bandwidth: %d sent, %d received", sm.sentBytes, sm.receivedBytes)

		min := time.Since(sm.startTime).Minutes()
		lg.Printf("Average bandwidth per minute: %d sent, %d received",
			int(float64(sm.sentBytes)/min), int(float64(sm.receivedBytes)/min))
	}

	sim, ok := sm.controllerTokenToSim[token]
	return sim, ok
}

///////////////////////////////////////////////////////////////////////////
// SimDispatcher

type SimDispatcher struct {
	sm *SimManager
}

func (sd *SimDispatcher) GetWorldUpdate(token string, update *SimWorldUpdate) error {
	if sim, ok := sd.sm.ControllerTokenToSim(token); !ok {
		return ErrNoSimForControllerToken
	} else {
		return sim.GetWorldUpdate(token, update)
	}
}

func (sd *SimDispatcher) SignOff(token string, _ *struct{}) error {
	if sim, ok := sd.sm.ControllerTokenToSim(token); !ok {
		return ErrNoSimForControllerToken
	} else {
		return sim.SignOff(token)
	}
}

type ChangeControlPositionArgs struct {
	ControllerToken string
	Callsign        string
	KeepTracks      bool
}

func (sd *SimDispatcher) ChangeControlPosition(cs *ChangeControlPositionArgs, _ *struct{}) error {
	if sim, ok := sd.sm.ControllerTokenToSim(cs.ControllerToken); !ok {
		return ErrNoSimForControllerToken
	} else {
		return sim.ChangeControlPosition(cs.ControllerToken, cs.Callsign, cs.KeepTracks)
	}
}

func (sd *SimDispatcher) TakeOrReturnLaunchControl(token string, _ *struct{}) error {
	if sim, ok := sd.sm.ControllerTokenToSim(token); !ok {
		return ErrNoSimForControllerToken
	} else {
		return sim.TakeOrReturnLaunchControl(token)
	}
}

type SetSimRateArgs struct {
	ControllerToken string
	Rate            float32
}

func (sd *SimDispatcher) SetSimRate(r *SetSimRateArgs, _ *struct{}) error {
	if sim, ok := sd.sm.controllerTokenToSim[r.ControllerToken]; !ok {
		return ErrNoSimForControllerToken
	} else {
		return sim.SetSimRate(r.ControllerToken, r.Rate)
	}
}

func (sd *SimDispatcher) TogglePause(token string, _ *struct{}) error {
	if sim, ok := sd.sm.ControllerTokenToSim(token); !ok {
		return ErrNoSimForControllerToken
	} else {
		return sim.TogglePause(token)
	}
}

type SetScratchpadArgs struct {
	ControllerToken string
	Callsign        string
	Scratchpad      string
}

func (sd *SimDispatcher) SetScratchpad(a *SetScratchpadArgs, _ *struct{}) error {
	if sim, ok := sd.sm.controllerTokenToSim[a.ControllerToken]; !ok {
		return ErrNoSimForControllerToken
	} else {
		return sim.SetScratchpad(a.ControllerToken, a.Callsign, a.Scratchpad)
	}
}

type InitiateTrackArgs AircraftSpecifier

func (sd *SimDispatcher) InitiateTrack(it *InitiateTrackArgs, _ *struct{}) error {
	if sim, ok := sd.sm.controllerTokenToSim[it.ControllerToken]; !ok {
		return ErrNoSimForControllerToken
	} else {
		return sim.InitiateTrack(it.ControllerToken, it.Callsign)
	}
}

type DropTrackArgs AircraftSpecifier

func (sd *SimDispatcher) DropTrack(dt *DropTrackArgs, _ *struct{}) error {
	if sim, ok := sd.sm.controllerTokenToSim[dt.ControllerToken]; !ok {
		return ErrNoSimForControllerToken
	} else {
		return sim.DropTrack(dt.ControllerToken, dt.Callsign)
	}
}

type HandoffArgs struct {
	ControllerToken string
	Callsign        string
	Controller      string
}

func (sd *SimDispatcher) HandoffTrack(h *HandoffArgs, _ *struct{}) error {
	if sim, ok := sd.sm.controllerTokenToSim[h.ControllerToken]; !ok {
		return ErrNoSimForControllerToken
	} else {
		return sim.HandoffTrack(h.ControllerToken, h.Callsign, h.Controller)
	}
}

func (sd *SimDispatcher) HandoffControl(h *HandoffArgs, _ *struct{}) error {
	if sim, ok := sd.sm.controllerTokenToSim[h.ControllerToken]; !ok {
		return ErrNoSimForControllerToken
	} else {
		return sim.HandoffControl(h.ControllerToken, h.Callsign)
	}
}

type AcceptHandoffArgs AircraftSpecifier

func (sd *SimDispatcher) AcceptHandoff(ah *AcceptHandoffArgs, _ *struct{}) error {
	if sim, ok := sd.sm.controllerTokenToSim[ah.ControllerToken]; !ok {
		return ErrNoSimForControllerToken
	} else {
		return sim.AcceptHandoff(ah.ControllerToken, ah.Callsign)
	}
}

type RejectHandoffArgs AircraftSpecifier

func (sd *SimDispatcher) RejectHandoff(rh *RejectHandoffArgs, _ *struct{}) error {
	if sim, ok := sd.sm.controllerTokenToSim[rh.ControllerToken]; !ok {
		return ErrNoSimForControllerToken
	} else {
		return sim.RejectHandoff(rh.ControllerToken, rh.Callsign)
	}
}

type CancelHandoffArgs AircraftSpecifier

func (sd *SimDispatcher) CancelHandoff(ch *CancelHandoffArgs, _ *struct{}) error {
	if sim, ok := sd.sm.controllerTokenToSim[ch.ControllerToken]; !ok {
		return ErrNoSimForControllerToken
	} else {
		return sim.CancelHandoff(ch.ControllerToken, ch.Callsign)
	}
}

type PointOutArgs struct {
	ControllerToken string
	Callsign        string
	Controller      string
}

func (sd *SimDispatcher) PointOut(po *PointOutArgs, _ *struct{}) error {
	if sim, ok := sd.sm.controllerTokenToSim[po.ControllerToken]; !ok {
		return ErrNoSimForControllerToken
	} else {
		return sim.PointOut(po.ControllerToken, po.Callsign, po.Controller)
	}
}

type AssignAltitudeArgs struct {
	ControllerToken string
	Callsign        string
	Altitude        int
}

func (sd *SimDispatcher) AssignAltitude(alt *AssignAltitudeArgs, _ *struct{}) error {
	if sim, ok := sd.sm.controllerTokenToSim[alt.ControllerToken]; !ok {
		return ErrNoSimForControllerToken
	} else {
		return sim.AssignAltitude(alt.ControllerToken, alt.Callsign, alt.Altitude)
	}
}

func (sd *SimDispatcher) SetTemporaryAltitude(alt *AssignAltitudeArgs, _ *struct{}) error {
	if sim, ok := sd.sm.controllerTokenToSim[alt.ControllerToken]; !ok {
		return ErrNoSimForControllerToken
	} else {
		return sim.SetTemporaryAltitude(alt.ControllerToken, alt.Callsign, alt.Altitude)
	}
}

type HeadingArgs struct {
	ControllerToken string
	Callsign        string
	Heading         int
	Present         bool
	LeftDegrees     int
	RightDegrees    int
	Turn            TurnMethod
}

func (sd *SimDispatcher) AssignHeading(hdg *HeadingArgs, _ *struct{}) error {
	if sim, ok := sd.sm.controllerTokenToSim[hdg.ControllerToken]; !ok {
		return ErrNoSimForControllerToken
	} else {
		return sim.AssignHeading(hdg)
	}
}

type SpeedArgs struct {
	ControllerToken string
	Callsign        string
	Speed           int
}

func (sd *SimDispatcher) AssignSpeed(sa *SpeedArgs, _ *struct{}) error {
	if sim, ok := sd.sm.controllerTokenToSim[sa.ControllerToken]; !ok {
		return ErrNoSimForControllerToken
	} else {
		return sim.AssignSpeed(sa.ControllerToken, sa.Callsign, sa.Speed)
	}
}

type FixArgs struct {
	ControllerToken string
	Callsign        string
	Fix             string
	Heading         int
	Altitude        int
	Speed           int
}

func (sd *SimDispatcher) DirectFix(f *FixArgs, _ *struct{}) error {
	if sim, ok := sd.sm.controllerTokenToSim[f.ControllerToken]; !ok {
		return ErrNoSimForControllerToken
	} else {
		return sim.DirectFix(f.ControllerToken, f.Callsign, f.Fix)
	}
}

func (sd *SimDispatcher) DepartFixHeading(f *FixArgs, _ *struct{}) error {
	if sim, ok := sd.sm.controllerTokenToSim[f.ControllerToken]; !ok {
		return ErrNoSimForControllerToken
	} else {
		return sim.DepartFixHeading(f.ControllerToken, f.Callsign, f.Fix, f.Heading)
	}
}

func (sd *SimDispatcher) CrossFixAt(f *FixArgs, _ *struct{}) error {
	if sim, ok := sd.sm.controllerTokenToSim[f.ControllerToken]; !ok {
		return ErrNoSimForControllerToken
	} else {
		return sim.CrossFixAt(f.ControllerToken, f.Callsign, f.Fix, f.Altitude, f.Speed)
	}
}

type ExpectApproachArgs struct {
	ControllerToken string
	Callsign        string
	Approach        string
}

func (sd *SimDispatcher) ExpectApproach(a *ExpectApproachArgs, _ *struct{}) error {
	if sim, ok := sd.sm.controllerTokenToSim[a.ControllerToken]; !ok {
		return ErrNoSimForControllerToken
	} else {
		return sim.ExpectApproach(a.ControllerToken, a.Callsign, a.Approach)
	}
}

type ClearedApproachArgs struct {
	ControllerToken string
	Callsign        string
	Approach        string
	StraightIn      bool
}

func (sd *SimDispatcher) ClearedApproach(c *ClearedApproachArgs, _ *struct{}) error {
	if sim, ok := sd.sm.controllerTokenToSim[c.ControllerToken]; !ok {
		return ErrNoSimForControllerToken
	} else {
		return sim.ClearedApproach(c.ControllerToken, c.Callsign, c.Approach, c.StraightIn)
	}
}

type GoAroundArgs AircraftSpecifier

func (sd *SimDispatcher) GoAround(ga *GoAroundArgs, _ *struct{}) error {
	if sim, ok := sd.sm.controllerTokenToSim[ga.ControllerToken]; !ok {
		return ErrNoSimForControllerToken
	} else {
		return sim.GoAround(ga.ControllerToken, ga.Callsign)
	}
}

type DeleteAircraftArgs AircraftSpecifier

func (sd *SimDispatcher) DeleteAircraft(da *DeleteAircraftArgs, _ *struct{}) error {
	if sim, ok := sd.sm.controllerTokenToSim[da.ControllerToken]; !ok {
		return ErrNoSimForControllerToken
	} else {
		return sim.DeleteAircraft(da.ControllerToken, da.Callsign)
	}
}

type AircraftCommandsError struct {
	Err       error
	Remaining []string
}

func (ace *AircraftCommandsError) Unwrap() error { return ace.Err }

func (e AircraftCommandsError) Error() string {
	s := e.Err.Error()
	if len(e.Remaining) > 0 {
		s += " remaining: " + strings.Join(e.Remaining, " ")
	}
	return s
}

func (sd *SimDispatcher) RunAircraftCommands(cmds *AircraftCommandsArgs, _ *struct{}) error {
	sim, ok := sd.sm.controllerTokenToSim[cmds.ControllerToken]
	if !ok {
		return ErrNoSimForControllerToken
	}

	commands := strings.Fields(cmds.Commands)

	for i, command := range commands {
		wrapError := func(e error) error {
			return &AircraftCommandsError{
				Err:       e,
				Remaining: commands[i:],
			}
		}

		switch command[0] {
		case 'D':
			if components := strings.Split(command, "/"); len(components) > 1 {
				// Depart <fix> at heading <hdg>
				fix := components[0][1:]

				if components[1][0] != 'H' {
					return wrapError(ErrInvalidCommandSyntax)
				}
				if hdg, err := strconv.Atoi(components[1][1:]); err != nil {
					return wrapError(err)
				} else if err := sim.DepartFixHeading(cmds.ControllerToken, cmds.Callsign, fix, hdg); err != nil {
					return wrapError(err)
				}
			} else if len(command) > 1 && command[1] >= '0' && command[1] <= '9' {
				// Looks like an altitude.
				if alt, err := strconv.Atoi(command[1:]); err != nil {
					return wrapError(err)
				} else if err := sim.AssignAltitude(cmds.ControllerToken, cmds.Callsign, 100*alt); err != nil {
					return wrapError(err)
				}
			} else if _, ok := sim.World.Locate(string(command[1:])); ok {
				if err := sim.DirectFix(cmds.ControllerToken, cmds.Callsign, command[1:]); err != nil {
					return wrapError(err)
				}
			} else {
				return wrapError(ErrInvalidCommandSyntax)
			}

		case 'H':
			if len(command) == 1 {
				if err := sim.AssignHeading(&HeadingArgs{
					ControllerToken: cmds.ControllerToken,
					Callsign:        cmds.Callsign,
					Present:         true,
				}); err != nil {
					return wrapError(err)
				}
			} else if hdg, err := strconv.Atoi(command[1:]); err != nil {
				return wrapError(err)
			} else if err := sim.AssignHeading(&HeadingArgs{
				ControllerToken: cmds.ControllerToken,
				Callsign:        cmds.Callsign,
				Heading:         hdg,
				Turn:            TurnClosest,
			}); err != nil {
				return wrapError(err)
			}

		case 'L':
			if l := len(command); l > 2 && command[l-1] == 'D' {
				// turn left x degrees
				if deg, err := strconv.Atoi(command[1 : l-1]); err != nil {
					return wrapError(err)
				} else if err := sim.AssignHeading(&HeadingArgs{
					ControllerToken: cmds.ControllerToken,
					Callsign:        cmds.Callsign,
					LeftDegrees:     deg,
				}); err != nil {
					return wrapError(err)
				}
			} else {
				// turn left heading...
				if hdg, err := strconv.Atoi(command[1:]); err != nil {
					return wrapError(err)
				} else if err := sim.AssignHeading(&HeadingArgs{
					ControllerToken: cmds.ControllerToken,
					Callsign:        cmds.Callsign,
					Heading:         hdg,
					Turn:            TurnLeft,
				}); err != nil {
					return wrapError(err)
				}
			}

		case 'R':
			if l := len(command); l > 2 && command[l-1] == 'D' {
				// turn right x degrees
				if deg, err := strconv.Atoi(command[1 : l-1]); err != nil {
					return wrapError(err)
				} else if err := sim.AssignHeading(&HeadingArgs{
					ControllerToken: cmds.ControllerToken,
					Callsign:        cmds.Callsign,
					RightDegrees:    deg,
				}); err != nil {
					return wrapError(err)
				}
			} else {
				// turn right heading...
				if hdg, err := strconv.Atoi(command[1:]); err != nil {
					return wrapError(err)
				} else if err := sim.AssignHeading(&HeadingArgs{
					ControllerToken: cmds.ControllerToken,
					Callsign:        cmds.Callsign,
					Heading:         hdg,
					Turn:            TurnRight,
				}); err != nil {
					return wrapError(err)
				}
			}

		case 'C', 'A':
			if len(command) > 4 && command[:3] == "CSI" && !isAllNumbers(command[3:]) {
				// Cleared straight in approach.
				if err := sim.ClearedApproach(cmds.ControllerToken, cmds.Callsign, command[3:], true); err != nil {
					return wrapError(err)
				}
			} else if command[0] == 'C' && len(command) > 2 && !isAllNumbers(command[1:]) {
				if components := strings.Split(command, "/"); len(components) > 1 {
					// Cross fix [at altitude] [at speed]
					fix := components[0][1:]
					alt, speed := 0, 0

					for _, cmd := range components[1:] {
						if len(cmd) == 0 {
							return wrapError(ErrInvalidCommandSyntax)
						}

						var err error
						if cmd[0] == 'A' {
							if alt, err = strconv.Atoi(cmd[1:]); err != nil {
								return wrapError(err)
							}
						} else if cmd[0] == 'S' {
							if speed, err = strconv.Atoi(cmd[1:]); err != nil {
								return wrapError(err)
							}
						} else {
							return wrapError(ErrInvalidCommandSyntax)
						}
					}

					if err := sim.CrossFixAt(cmds.ControllerToken, cmds.Callsign, fix, 100*alt, speed); err != nil {
						return wrapError(err)
					}
				} else if err := sim.ClearedApproach(cmds.ControllerToken, cmds.Callsign, command[1:], false); err != nil {
					return wrapError(err)
				}
			} else {
				// Otherwise look for an altitude
				if alt, err := strconv.Atoi(command[1:]); err != nil {
					return wrapError(err)
				} else if err := sim.AssignAltitude(cmds.ControllerToken, cmds.Callsign, 100*alt); err != nil {
					return wrapError(err)
				}
			}

		case 'S':
			if len(command) == 1 {
				// Cancel speed restrictions
				if err := sim.AssignSpeed(cmds.ControllerToken, cmds.Callsign, 0); err != nil {
					return wrapError(err)
				}
			} else {
				if kts, err := strconv.Atoi(command[1:]); err != nil {
					return wrapError(err)
				} else if err := sim.AssignSpeed(cmds.ControllerToken, cmds.Callsign, kts); err != nil {
					return wrapError(err)
				}
			}

		case 'E':
			// Expect approach.
			if len(command) > 1 {
				if err := sim.ExpectApproach(cmds.ControllerToken, cmds.Callsign, command[1:]); err != nil {
					return wrapError(err)
				}
			} else {
				return wrapError(ErrInvalidCommandSyntax)
			}

		default:
			return wrapError(ErrInvalidCommandSyntax)
		}
	}
	return nil
}

type LaunchAircraftArgs struct {
	ControllerToken string
	Aircraft        Aircraft
}

func (sd *SimDispatcher) LaunchAircraft(ls *LaunchAircraftArgs, _ *struct{}) error {
	sim, ok := sd.sm.controllerTokenToSim[ls.ControllerToken]
	if !ok {
		return ErrNoSimForControllerToken
	}
	sim.LaunchAircraft(ls.Aircraft)
	return nil
}

func RunSimServer() {
	l, err := net.Listen("tcp", ":8000")
	if err != nil {
		lg.Errorf("tcp listen: %v", err)
		return
	}

	// If we're just running the server, we don't care about the returned
	// configs...
	runServer(l, false)
}

func TryConnectRemoteServer(hostname string) (chan *SimServer, error) {
	client, err := rpc.DialHTTP("tcp", hostname)
	if err != nil {
		return nil, err
	}

	ch := make(chan *SimServer, 1)
	go func() {
		var configs map[string]*SimConfiguration
		if err := client.Call("SimManager.GetSimConfigurations", 0, &configs); err != nil {
			close(ch)
			lg.Errorf("%v", err)
		} else {
			ch <- &SimServer{
				name:    "Network (Multi-controller)",
				client:  client,
				configs: configs,
			}
		}
	}()

	return ch, nil
}

func LaunchLocalSimServer() (chan *SimServer, error) {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		return nil, err
	}

	port := l.Addr().(*net.TCPAddr).Port

	configsChan := runServer(l, true)

	ch := make(chan *SimServer, 1)
	go func() {
		configs := <-configsChan

		client, err := rpc.DialHTTP("tcp", fmt.Sprintf("localhost:%d", port))
		if err != nil {
			lg.Errorf("%v", err)
			os.Exit(1)
		}

		ch <- &SimServer{
			name:    "Local (Single controller)",
			client:  client,
			configs: configs,
		}
	}()

	return ch, nil
}

func runServer(l net.Listener, isLocal bool) chan map[string]*SimConfiguration {
	ch := make(chan map[string]*SimConfiguration, 1)

	server := func() {
		var e ErrorLogger
		scenarioGroups, simConfigurations := LoadScenarioGroups(&e)
		if e.HaveErrors() {
			e.PrintErrors()
			os.Exit(1)
		}

		// Filter the scenarios and configs: for local, we only want ones
		// with solo_controller specified, and for the remote server, we
		// only want the ones with multi_controllers.

		sm := NewSimManager(scenarioGroups, simConfigurations)
		rpc.Register(sm)
		rpc.RegisterName("Sim", &SimDispatcher{sm: sm})
		rpc.HandleHTTP()

		ch <- simConfigurations

		lg.Printf("Listening on %+v", l)
		http.Serve(MakeLoggingListener(l, &sm.sentBytes, &sm.receivedBytes), nil) // noreturn
	}

	if isLocal {
		go server()
	} else {
		server()
	}
	return ch
}