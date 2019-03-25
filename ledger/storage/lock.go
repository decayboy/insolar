//
// Copyright 2019 Insolar Technologies GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package storage

import (
	"fmt"
	"sync"

	"github.com/insolar/insolar/insolar"
)

type mucount struct {
	*sync.RWMutex
	count int32
}

//go:generate minimock -i github.com/insolar/insolar/ledger/storage.IDLocker -o ./ -s _mock.go
type IDLocker interface {
	Lock(id *insolar.ID)
	Unlock(id *insolar.ID)

	RLock(id *insolar.ID)
	RUnlock(id *insolar.ID)
}

// IDLocker provides Lock/Unlock methods per record ID.
//
// TODO: for further optimization we could use sync.Pool for mutexes.
type idLocker struct {
	muxs map[insolar.ID]*mucount
	mu   sync.Mutex
}

// NewIDLocker creates new initialized IDLocker.
func NewIDLocker() IDLocker {
	return &idLocker{
		muxs: make(map[insolar.ID]*mucount),
	}
}

// Lock locks mutex belonged to record ID.
// If mutex does not exist, it will be created in concurrent safe fashion.
func (l *idLocker) Lock(id *insolar.ID) {
	l.mu.Lock()
	mc, ok := l.muxs[*id]
	if !ok {
		mc = &mucount{RWMutex: &sync.RWMutex{}}
		l.muxs[*id] = mc
	}
	mc.count++
	l.mu.Unlock()

	mc.Lock()
}

// Unlock unlocks mutex belonged to record ID.
func (l *idLocker) Unlock(id *insolar.ID) {
	l.mu.Lock()
	defer l.mu.Unlock()

	mc, ok := l.muxs[*id]
	if !ok {
		panic(fmt.Sprintf("try to unlock not initialized mutex for ID %+v", id))
	}
	mc.count--
	mc.Unlock()
	if mc.count == 0 {
		delete(l.muxs, *id)
	}
}

// Lock locks mutex belonged to record ID.
// If mutex does not exist, it will be created in concurrent safe fashion.
func (l *idLocker) RLock(id *insolar.ID) {
	l.mu.Lock()
	mc, ok := l.muxs[*id]
	if !ok {
		mc = &mucount{RWMutex: &sync.RWMutex{}}
		l.muxs[*id] = mc
	}
	mc.count++
	l.mu.Unlock()

	mc.RLock()
}

// RUnlock unlocks rwmutex belonged to record ID.
func (l *idLocker) RUnlock(id *insolar.ID) {
	l.mu.Lock()
	defer l.mu.Unlock()

	mc, ok := l.muxs[*id]
	if !ok {
		panic(fmt.Sprintf("try to unlock not initialized mutex for ID %+v", id))
	}
	mc.count--
	mc.RUnlock()
	if mc.count == 0 {
		delete(l.muxs, *id)
	}
}
