//
//    Copyright 2019 Insolar Technologies
//
//    Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at
//
//        http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and
//    limitations under the License.
//

package smachine

import (
	"math"
)

// ConditionalBool allows Acquire() call to pass through when current value is >0
func NewConditional(initial int, name string) ConditionalLink {
	ctl := &conditionalSync{}
	ctl.controller.Init(name)
	deps, _ := ctl.AdjustLimit(initial, false)
	if len(deps) != 0 {
		panic("illegal state")
	}
	return ConditionalLink{ctl}
}

type ConditionalLink struct {
	ctl *conditionalSync
}

func (v ConditionalLink) IsZero() bool {
	return v.ctl == nil
}

// Creates an adjustment that alters the conditional's value when the adjustment is applied with SynchronizationContext.ApplyAdjustment()
// Can be applied multiple times.
func (v ConditionalLink) NewDelta(delta int) SyncAdjustment {
	if v.ctl == nil {
		panic("illegal state")
	}
	return SyncAdjustment{controller: v.ctl, adjustment: delta, isAbsolute: false}
}

// Creates an adjustment that sets the given value when applied with SynchronizationContext.ApplyAdjustment()
// Can be applied multiple times.
func (v ConditionalLink) NewValue(value int) SyncAdjustment {
	if v.ctl == nil {
		panic("illegal state")
	}
	return SyncAdjustment{controller: v.ctl, adjustment: value, isAbsolute: true}
}

func (v ConditionalLink) SyncLink() SyncLink {
	return NewSyncLink(v.ctl)
}

type conditionalSync struct {
	controller holdingQueueController
}

func (p *conditionalSync) CheckState() Decision {
	if p.controller.isOpen() {
		return Passed
	}
	return NotPassed
}

func (p *conditionalSync) CheckDependency(dep SlotDependency) Decision {
	if entry, ok := dep.(*dependencyQueueEntry); ok {
		switch {
		case !entry.link.IsValid(): // just to make sure
			return Impossible
		case !p.controller.Contains(entry):
			return Impossible
		case p.controller.isOpen():
			return Passed
		default:
			return NotPassed
		}
	}
	return Impossible
}

func (p *conditionalSync) UseDependency(dep SlotDependency, flags SlotDependencyFlags) (Decision, SlotDependency) {
	if entry, ok := dep.(*dependencyQueueEntry); ok {
		switch {
		case !entry.link.IsValid(): // just to make sure
			return Impossible, nil
		case !entry.IsCompatibleWith(flags):
			return Impossible, nil
		case !p.controller.Contains(entry):
			return Impossible, nil
		case p.controller.isOpen():
			return Passed, nil
		default:
			return NotPassed, nil
		}
	}
	return Impossible, nil
}

func (p *conditionalSync) CreateDependency(holder SlotLink, flags SlotDependencyFlags) (BoolDecision, SlotDependency) {
	if p.controller.isOpen() {
		return true, nil
	}
	return false, p.controller.queue.AddSlot(holder, flags)
}

func (p *conditionalSync) GetLimit() (limit int, isAdjustable bool) {
	return p.controller.state, true
}

func (p *conditionalSync) AdjustLimit(limit int, absolute bool) ([]StepLink, bool) {
	if ok, newState := applyWrappedAdjustment(p.controller.state, limit, math.MinInt32, math.MaxInt32, absolute); ok {
		return p.setLimit(newState)
	}
	return nil, false
}

func (p *conditionalSync) setLimit(limit int) ([]StepLink, bool) {
	p.controller.state = limit
	if !p.controller.isOpen() {
		return nil, false
	}
	return p.controller.queue.FlushAllAsLinks(), true
}

func (p *conditionalSync) GetCounts() (active, inactive int) {
	return -1, p.controller.queue.Count()
}

func (p *conditionalSync) GetName() string {
	return p.controller.GetName()
}

type holdingQueueController struct {
	waitingQueueController
	state int
}

func (p *holdingQueueController) isOpen() bool {
	return p.state > 0
}

func (p *holdingQueueController) IsOpen(SlotDependency) bool {
	return p.isOpen()
}

func (p *holdingQueueController) Release(link SlotLink, flags SlotDependencyFlags, removeFn func()) ([]PostponedDependency, []StepLink) {
	removeFn()
	if p.isOpen() && p.queue.Count() > 0 {
		panic("illegal state")
	}
	return nil, nil
}

func (p *holdingQueueController) IsReleaseOnWorking(SlotLink, SlotDependencyFlags) bool {
	return p.isOpen()
}

func (p *holdingQueueController) IsReleaseOnStepping(_ SlotLink, flags SlotDependencyFlags) bool {
	return flags&syncForOneStep != 0 || p.isOpen()
}

func applyWrappedAdjustment(current, adjustment, min, max int, absolute bool) (bool, int) {
	if absolute {
		if current == adjustment {
			return false, current
		}
		if adjustment < min {
			return true, min
		}
		if adjustment > max {
			return true, max
		}
		return true, adjustment
	}

	if adjustment == 0 {
		return false, current
	}
	if adjustment < 0 {
		adjustment += current
		if adjustment < min || adjustment > current /* overflow */ {
			return true, min
		}
		return true, adjustment
	}

	adjustment += current
	if adjustment > max || adjustment < current /* overflow */ {
		return true, max
	}
	return true, adjustment
}
