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

package agentid_test

import (
	"testing"

	"github.com/GoogleCloudPlatform/ubbagent/agentid"
	"github.com/GoogleCloudPlatform/ubbagent/persistence"
)

func TestCreateOrGet(t *testing.T) {
	p1 := persistence.NewMemoryPersistence()
	p2 := persistence.NewMemoryPersistence()

	id1, err := agentid.CreateOrGet(p1)
	if err != nil {
		t.Fatalf("error creating agentid: %+v", err)
	}
	id2, err := agentid.CreateOrGet(p2)
	if err != nil {
		t.Fatalf("error creating agentid: %+v", err)
	}
	id1Again, err := agentid.CreateOrGet(p1)
	if err != nil {
		t.Fatalf("error creating agentid: %+v", err)
	}

	if id1 == id2 {
		t.Fatalf("agentid.CreateOrGet should have created unique IDs, but both were %v", id1)
	}
	if id1 != id1Again {
		t.Fatalf("agentid.CreateOrGet returned same ID for same persistence, but got different IDs: %v, %v", id1, id1Again)
	}
}
