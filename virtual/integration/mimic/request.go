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

package mimic

import (
	"github.com/insolar/insolar/insolar"
	"github.com/insolar/insolar/insolar/record"
)

type RequestStatus int

const (
	RequestRegistered RequestStatus = iota
	RequestFinished
)

type RequestEntity struct {
	ID        insolar.ID
	Status    RequestStatus
	Request   record.Request
	Outgoings map[insolar.ID]*RequestEntity
	Result    []byte
	ResultID  insolar.ID
}

// TODO[bigbes]: support deduplication here
func (e *RequestEntity) appendOutgoing(outgoingEntity *RequestEntity) {
	e.Outgoings[outgoingEntity.ID] = outgoingEntity
}

func (e RequestEntity) hasOpenedOutgoings() bool { //nolint: unused
	for _, req := range e.Outgoings {
		if req.Status != RequestFinished {
			return true
		}
	}

	return false
}

func (e *RequestEntity) getPulse() insolar.PulseNumber {
	return e.ID.Pulse()
}

func NewIncomingRequestEntity(requestID insolar.ID, request record.Request) *RequestEntity {
	_, ok := request.(*record.IncomingRequest)
	if !ok {
		return nil
	}
	return &RequestEntity{
		Status:    RequestRegistered,
		Result:    nil,
		Request:   request.(*record.IncomingRequest),
		ID:        requestID,
		Outgoings: make(map[insolar.ID]*RequestEntity),
	}
}

func NewOutgoingRequestEntity(requestID insolar.ID, request record.Request) *RequestEntity {
	_, ok := request.(*record.OutgoingRequest)
	if !ok {
		return nil
	}
	return &RequestEntity{
		Status:    RequestRegistered,
		Result:    nil,
		Request:   request.(*record.IncomingRequest),
		ID:        requestID,
		Outgoings: nil,
	}
}