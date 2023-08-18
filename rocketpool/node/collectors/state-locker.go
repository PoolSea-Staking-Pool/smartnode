package collectors

import (
	"math/big"
	"sync"

	"github.com/Seb369888/smartnode/shared/services/state"
)

type StateLocker struct {
	state               *state.NetworkState
	totalEffectiveStake *big.Int

	// Internal fields
	lock *sync.Mutex
}

func NewStateLocker() *StateLocker {
	return &StateLocker{
		lock: &sync.Mutex{},
	}
}

func (l *StateLocker) UpdateState(state *state.NetworkState, totalEffectiveStake *big.Int) {
	l.lock.Lock()
	defer l.lock.Unlock()
	l.state = state
	if totalEffectiveStake != nil {
		l.totalEffectiveStake = totalEffectiveStake
	}
}

func (l *StateLocker) GetState() *state.NetworkState {
	l.lock.Lock()
	defer l.lock.Unlock()
	return l.state
}

func (l *StateLocker) GetTotalEffectiveRPLStake() *big.Int {
	l.lock.Lock()
	defer l.lock.Unlock()
	return l.totalEffectiveStake
}
