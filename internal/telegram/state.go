package telegram

import (
	"github.com/WhiteRabbit21m/sats4sats_light-wallet/internal/lnbits"
	"github.com/WhiteRabbit21m/sats4sats_light-wallet/internal/telegram/intercept"
)

type StateCallbackMessage map[lnbits.UserStateKey]func(ctx intercept.Context) (intercept.Context, error)

var stateCallbackMessage StateCallbackMessage

func initializeStateCallbackMessage(bot *TipBot) {
	stateCallbackMessage = StateCallbackMessage{
		lnbits.UserStateLNURLEnterAmount: bot.enterAmountHandler,
		lnbits.UserEnterAmount:           bot.enterAmountHandler,
		lnbits.UserEnterUser:             bot.enterUserHandler,
	}
}
