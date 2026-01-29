package admin

import (
	"github.com/WhiteRabbit21m/sats4sats_light-wallet/internal/telegram"
)

type Service struct {
	bot *telegram.TipBot
}

func New(b *telegram.TipBot) Service {
	return Service{
		bot: b,
	}
}
