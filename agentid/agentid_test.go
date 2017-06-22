package agentid_test

import (
	"testing"
	"ubbagent/agentid"
	"ubbagent/persistence"
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
