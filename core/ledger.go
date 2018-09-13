/*
 *    Copyright 2018 Insolar
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

package core

// Ledger is the global ledger handler. Other system parts communicate with ledger through it.
type Ledger interface {
	// GetArtifactManager returns artifact manager to work with.
	GetArtifactManager() ArtifactManager

	// GetJetCoordinator returns jet coordinator to work with.
	GetJetCoordinator() JetCoordinator

	// GetPulseManager returns pulse manager to work with.
	GetPulseManager() PulseManager
}
