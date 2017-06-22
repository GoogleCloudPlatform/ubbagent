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
