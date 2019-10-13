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
	"context"

	"github.com/pkg/errors"

	"github.com/insolar/insolar/insolar"
	"github.com/insolar/insolar/insolar/gen"
	"github.com/insolar/insolar/insolar/record"
	"github.com/insolar/insolar/ledger/artifact"
)

type client struct {
	storage Storage
}

func NewClient(storage Storage) artifact.Manager {
	return &client{storage: storage}
}

func (c *client) GetObject(ctx context.Context, head insolar.Reference) (artifact.ObjectDescriptor, error) {
	objectID := *head.GetLocal()
	state, index, _, err := c.storage.GetObject(ctx, objectID)
	if err != nil {
		return nil, err
	}

	return &objectDescriptor{
		head:        head,
		state:       *index.Lifeline.LatestState,
		prototype:   state.GetImage(),
		isPrototype: state.GetIsPrototype(),
		parent:      index.Lifeline.Parent,
		memory:      state.GetMemory(),
	}, nil
}

func (c *client) ActivateObject(
	ctx context.Context,
	domain, objectRef, parent, prototype insolar.Reference,
	memory []byte,
) error {
	objectID := *objectRef.GetLocal()

	mimicStorage := c.storage.(*mimicStorage)
	mimicStorage.Requests[objectID] = NewIncomingRequestEntity(objectID, &record.IncomingRequest{
		CallType: record.CTGenesis,
		Method:   objectRef.String(),
	})
	mimicStorage.Objects[objectID] = &ObjectEntity{
		ObjectChanges: nil,
		RequestsMap:   make(map[insolar.ID]*RequestEntity),
		RequestsList:  nil,
	}

	rec := record.Activate{
		Request:     objectRef,
		Memory:      memory,
		Image:       prototype,
		IsPrototype: false,
		Parent:      parent,
	}

	_, err := c.storage.SetResult(ctx, &record.Result{
		Request: objectRef,
		Payload: []byte{},
	})
	if err != nil {
		return errors.Wrap(err, "failed to store result")
	}

	return c.storage.SetObject(ctx, *objectRef.GetLocal(), &rec, *objectRef.GetLocal())
}

// FORCEFULLY DISABLED
func (c *client) RegisterRequest(ctx context.Context, req record.IncomingRequest) (*insolar.ID, error) {
	requestID := gen.ID()
	return &requestID, nil
}

// FORCEFULLY DISABLED
func (c *client) RegisterResult(ctx context.Context, obj, request insolar.Reference, payload []byte) (*insolar.ID, error) {
	return nil, nil
}

// NOT NEEDED
func (c client) UpdateObject(ctx context.Context, domain, request insolar.Reference, obj artifact.ObjectDescriptor, memory []byte) error {
	var (
		image *insolar.Reference
		err   error
	)
	if obj.IsPrototype() {
		image, err = obj.Code()
	} else {
		image, err = obj.Prototype()
	}
	if err != nil {
		return errors.Wrap(err, "failed to update object")
	}

	rec := record.Amend{
		Request:     request,
		Memory:      memory,
		Image:       *image,
		IsPrototype: obj.IsPrototype(),
		PrevState:   *obj.StateID(),
	}

	objectID := *obj.HeadRef().GetLocal()
	return c.storage.SetObject(ctx, insolar.ID{}, &rec, objectID)
}

// NOT NEEDED
func (c client) DeployCode(
	ctx context.Context,
	domain insolar.Reference,
	request insolar.Reference,
	code []byte,
	machineType insolar.MachineType,
) (*insolar.ID, error) {
	panic("implement me")
}
