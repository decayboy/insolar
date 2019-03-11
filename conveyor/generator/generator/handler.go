/*
 *    Copyright 2019 Insolar Technologies
 *
 *    Licensed under the Apache License, Version 2.0 (the "License");
 *    you may not use this file except in compliance with the License.
 *    You may obtain a copy of the License at
 *
 *        http://www.apache.org/licenses/LICENSE-2.0
 *
 *    Unless required by applicable law or agreed to in writing, software
 *    distributed under the License is distributed on an "AS IS" BASIS,
 *    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *    See the License for the specific language governing permissions and
 *    limitations under the License.
 */

package generator

// handler struct for code generation of raw handlers
type handler struct {
	machine *stateMachine
	state   int
	name    string
	params  []string
	results []string
}

func (h *handler) setAsState() {
	if len(h.params) != 0 {
		exitWithError("%s state must don't have any parameters", h.name)
	}
	if len(h.results) != 1 || h.results[0] != "common.ElState" {
		exitWithError("%s state should returns only common.ElState")
	}
	h.machine.States = append(h.machine.States, state{Name: h.name})
}

func (h *handler) setAsInit() {
	h.checkInitHandler()
	if h.machine.States[0].Transition != nil {
		exitWithError("%s init handler already exists", h.machine.Name)
	}
	h.machine.States[0].Transition = h
}

func (h *handler) setAsInitFuture() {
	h.checkInitHandler()
	if h.machine.States[0].TransitionFuture != nil {
		exitWithError("%s init (future) handler already exists", h.machine.Name)
	}
	h.machine.States[0].TransitionFuture = h
}

func (h *handler) setAsInitPast() {
	h.checkInitHandler()
	if h.machine.States[0].TransitionPast != nil {
		exitWithError("%s init (past) handler already exists", h.machine.Name)
	}
	h.machine.States[0].TransitionPast = h
}

func (h *handler) setAsErrorState() {
	h.checkErrorStateHandler()
	h.machine.States[h.state].ErrorState = h
}

func (h *handler) setAsErrorStateFuture() {
	h.checkErrorStateHandler()
	h.machine.States[h.state].ErrorStateFuture = h
}

func (h *handler) setAsErrorStatePast() {
	h.checkErrorStateHandler()
	h.machine.States[h.state].ErrorStatePast = h
}

func (h *handler) setAsMigration() {
	h.checkMigrationHandler()
	h.machine.States[h.state].Migration = h
}

func (h *handler) setAsMigrationFuturePresent() {
	h.checkMigrationHandler()
	h.machine.States[h.state].MigrationFuturePresent = h
}

func (h *handler) setAsTransition() {
	h.checkTransitionHandler()
	h.machine.States[h.state].Transition = h
}

func (h *handler) setAsTransitionFuture() {
	h.checkTransitionHandler()
	h.machine.States[h.state].TransitionFuture = h
}

func (h *handler) setAsTransitionPast() {
	h.checkTransitionHandler()
	h.machine.States[h.state].TransitionPast = h
}

func (h *handler) setAsAdapterResponse() {
	h.checkAdapterResponseHandler()
	h.machine.States[h.state].AdapterResponse = h
}

func (h *handler) setAsAdapterResponseFuture() {
	h.checkAdapterResponseHandler()
	h.machine.States[h.state].AdapterResponseFuture = h
}

func (h *handler) setAsAdapterResponsePast() {
	h.checkAdapterResponseHandler()
	h.machine.States[h.state].AdapterResponsePast = h
}

func (h *handler) setAsAdapterResponseError() {
	if len(h.params) != 4 {
		exitWithError("%s must have four parameters", h.name)
	}
	h.checkInterfaceParameter(0)
	h.checkInterfaceParameter(1)
	h.checkAdapterResponseParameter(2)
	h.checkErrorParameter(3)
	h.checkErrorHandlerReturns()
	h.machine.States[h.state].AdapterResponseError = h
}

func (h *handler) setAsAdapterResponseErrorFuture() {
	if len(h.params) != 4 {
		exitWithError("%s must have four parameters", h.name)
	}
	h.checkInterfaceParameter(0)
	h.checkInterfaceParameter(1)
	h.checkAdapterResponseParameter(2)
	h.checkErrorParameter(3)
	h.checkErrorHandlerReturns()
	h.machine.States[h.state].AdapterResponseErrorFuture = h
}

func (h *handler) setAsAdapterResponseErrorPast() {
	if len(h.params) != 4 {
		exitWithError("%s must have four parameters", h.name)
	}
	h.checkInterfaceParameter(0)
	h.checkInterfaceParameter(1)
	h.checkAdapterResponseParameter(2)
	h.checkErrorParameter(3)
	h.checkErrorHandlerReturns()
	h.machine.States[h.state].AdapterResponseErrorPast = h
}
