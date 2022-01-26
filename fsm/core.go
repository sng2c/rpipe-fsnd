package fsm

import (
	log "github.com/sirupsen/logrus"
	"sync"
)

type State string
type Event string
type RouteMap map[Event]State
type Fsm map[State]RouteMap
type Instance struct {
	Fsm   Fsm
	State State
	mu    sync.Mutex
}

func NewFsm() Fsm {
	return make(Fsm)
}

func (m Fsm) Append(state0 State, event Event, state1 State) {
	if _, ok := m[state0]; !ok {
		m[state0] = make(RouteMap)
	}
	m[state0][event] = state1
}
func (m Fsm) LogDump() {
	for k, v := range m {
		log.Debugln(k)
		for kk, vv := range v {
			log.Debugln("\t", kk, vv)
		}
	}
}
func (m Fsm) NewInstance(init State) Instance {
	return Instance{
		Fsm:   m,
		State: init,
	}
}

func (inst *Instance) Emit(event Event) bool {


	newState, ok := inst.Fsm[inst.State][event]
	if !ok {
		return false
	}

	// Lock
	inst.mu.Lock()
	inst.State = newState
	// Unlock
	inst.mu.Unlock()
	return true
}
