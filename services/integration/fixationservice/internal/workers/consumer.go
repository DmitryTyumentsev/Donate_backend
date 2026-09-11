package workers

import (
	"Broker_backend/shared/pkg/authz"
	"context"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

func Consumer(){
	s := NewService(cfg, zap.New(), clock, fixation, tx)
	ctx := context.Background()
	ctx = authz.WithPrincipal(ctx, authz.Principal{
		AgencyID: s.cfg.Business., //думаю откуда брать принципал(чей брать). с одной стороны - это сторонняя система мы точно не должны прикладывать принципал из нашей системы, выходит мы должны отправить принципал по которому ходим в амо, а он должен лежать в конфигах?
		UserID:   uuid.UUID{},
		DeviceID: "",
		Role:     "",
	})

}
