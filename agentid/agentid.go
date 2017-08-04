// Copyright 2017 Google Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package agentid

import (
	"github.com/google/uuid"
	"ubbagent/persistence"
)

const agentIdKey = "agentid"

type idHolder struct {
	AgentId string
}

func CreateOrGet(p persistence.Persistence) (string, error) {
	holder := idHolder{}
	err := p.Load(agentIdKey, &holder)
	if err != nil && err != persistence.ErrNotFound {
		return "", err
	}
	if err == persistence.ErrNotFound {
		id, err := uuid.NewRandom()
		if err != nil {
			return "", err
		}
		holder.AgentId = id.String()
		err = p.Store(agentIdKey, &holder)
		if err != nil {
			return "", err
		}
	}
	return holder.AgentId, nil
}
