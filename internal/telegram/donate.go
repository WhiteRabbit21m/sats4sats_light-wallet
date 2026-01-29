package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/fiatjaf/go-lnurl"

	"github.com/WhiteRabbit21m/sats4sats_light-wallet/internal"
	"github.com/WhiteRabbit21m/sats4sats_light-wallet/internal/telegram/intercept"

	"github.com/WhiteRabbit21m/sats4sats_light-wallet/internal/errors"

	"github.com/WhiteRabbit21m/sats4sats_light-wallet/internal/str"

	"github.com/WhiteRabbit21m/sats4sats_light-wallet/internal/lnbits"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/lightningtipbot/telebot.v3"
)

// DONATION SYSTEM MODIFIED TO USE BOT CONFIGURATION
// DONATIONS NOW GO TO THE CONFIGURED BOT OWNER
// USERNAME AND HOSTNAME ARE READ FROM config.yaml
// DONATIONS ROUTE TO: username@hostname AS CONFIGURED

var (
	donationEndpoint string
)

func helpDonateUsage(ctx context.Context, errormsg string) string {
	hostname := internal.Configuration.Bot.Name
	if len(errormsg) > 0 {
		return fmt.Sprintf(Translate(ctx, "donateHelpText"), fmt.Sprintf("%s", errormsg), hostname, hostname)
	} else {
		return fmt.Sprintf(Translate(ctx, "donateHelpText"), "", hostname, hostname)
	}
}

func (bot TipBot) donationHandler(ctx intercept.Context) (intercept.Context, error) {
	// check and print all commands
	m := ctx.Message()
	bot.anyTextHandler(ctx)
	user := LoadUser(ctx)
	if user.Wallet == nil {
		return ctx, errors.Create(errors.UserNoWalletError)
	}
	// if no amount is in the command, ask for it
	amount, err := decodeAmountFromCommand(m.Text)
	if (err != nil || amount < 1) && m.Chat.Type == tb.ChatPrivate {
		// // no amount was entered, set user state and ask for amount
		_, err = bot.askForAmount(ctx, "", "CreateDonationState", 0, 0, m.Text)
		return ctx, err
	}
	amount = amount * 1000
	// command is valid
	msg := bot.trySendMessageEditable(m.Chat, Translate(ctx, "donationProgressMessage"))
	// get invoice
	r, err := http.NewRequest(http.MethodGet, donationEndpoint, nil)
	if err != nil {
		log.Errorln(err)
		bot.tryEditMessage(msg, Translate(ctx, "donationErrorMessage"))
		return ctx, err
	}
	// Create query parameters
	params := url.Values{}
	params.Set("amount", strconv.FormatInt(amount, 10))
	params.Set("comment", fmt.Sprintf("from %s bot %s", GetUserStr(user.Telegram), GetUserStr(bot.Telegram.Me)))
	// Set the query parameters in the URL
	r.URL.RawQuery = params.Encode()

	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		log.Errorln(err)
		bot.tryEditMessage(msg, Translate(ctx, "donationErrorMessage"))
		return ctx, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Errorln(err)
		bot.tryEditMessage(msg, Translate(ctx, "donationErrorMessage"))
		return ctx, err
	}
	pv := lnurl.LNURLPayValues{}
	err = json.Unmarshal(body, &pv)
	if err != nil {
		log.Errorln(err)
		bot.tryEditMessage(msg, Translate(ctx, "donationErrorMessage"))
		return ctx, err
	}
	if pv.Status == "ERROR" || len(pv.PR) < 1 {
		log.Errorln(err)
		bot.tryEditMessage(msg, Translate(ctx, "donationErrorMessage"))
		return ctx, err
	}

	// send donation invoice
	// user := LoadUser(ctx)
	// bot.trySendMessage(user.Telegram, string(body))
	_, err = user.Wallet.Pay(lnbits.PaymentParams{Out: true, Bolt11: string(pv.PR)}, bot.Client)
	if err != nil {
		userStr := GetUserStr(user.Telegram)
		errmsg := fmt.Sprintf("[/donate] Donation failed for user %s: %s", userStr, err)
		log.Errorln(errmsg)
		bot.tryEditMessage(msg, Translate(ctx, "donationErrorMessage"))
		return ctx, err
	}
	// hotfix because the edit doesn't work!
	// todo: fix edit
	// bot.tryEditMessage(msg, Translate(ctx, "donationSuccess"))
	bot.tryDeleteMessage(msg)
	bot.trySendMessage(m.Chat, Translate(ctx, "donationSuccess"))
	return ctx, nil
}

func init() {
	// Costruisce l'endpoint di donazione usando la configurazione del bot
	username := strings.TrimPrefix(internal.Configuration.Bot.Username, "@")
	hostname := internal.Configuration.Bot.Name

	// Costruisce l'URL: https://hostname/.well-known/lnurlp/username
	donationURL := fmt.Sprintf("https://%s/.well-known/lnurlp/%s", hostname, username)

	// Applica l'offuscamento ROT13 per mantenere la stessa struttura
	var sb strings.Builder
	_, err := io.Copy(&sb, rot13Reader{strings.NewReader(applyRot13(donationURL))})
	if err != nil {
		panic(err)
	}
	donationEndpoint = sb.String()
}

// applyRot13 applica la codifica ROT13 a una stringa
func applyRot13(s string) string {
	var result strings.Builder
	for _, c := range s {
		switch {
		case c >= 65 && c <= 90:
			if c <= 77 {
				result.WriteByte(byte(c + 13))
			} else {
				result.WriteByte(byte(c - 13))
			}
		case c >= 97 && c <= 122:
			if c <= 109 {
				result.WriteByte(byte(c + 13))
			} else {
				result.WriteByte(byte(c - 13))
			}
		default:
			result.WriteByte(byte(c))
		}
	}
	return result.String()
}

type rot13Reader struct {
	r io.Reader
}

func (rot13 rot13Reader) Read(b []byte) (int, error) {
	n, err := rot13.r.Read(b)
	for i := 0; i < n; i++ {
		switch {
		case b[i] >= 65 && b[i] <= 90:
			if b[i] <= 77 {
				b[i] = b[i] + 13
			} else {
				b[i] = b[i] - 13
			}
		case b[i] >= 97 && b[i] <= 122:
			if b[i] <= 109 {
				b[i] = b[i] + 13
			} else {
				b[i] = b[i] - 13
			}
		}
	}
	return n, err
}

func (bot TipBot) parseCmdDonHandler(ctx intercept.Context) error {
	m := ctx.Message()
	arg := ""
	if strings.HasPrefix(strings.ToLower(m.Text), "/send") {
		arg, _ = getArgumentFromCommand(m.Text, 2)
		if arg != "@"+bot.Telegram.Me.Username {
			return fmt.Errorf("err")
		}
	}
	if strings.HasPrefix(strings.ToLower(m.Text), "/tip") {
		arg = GetUserStr(m.ReplyTo.Sender)
		if arg != "@"+bot.Telegram.Me.Username {
			return fmt.Errorf("err")
		}
	}
	// Controlla se l'argomento corrisponde al username configurato del bot
	configuredUsername := internal.Configuration.Bot.Username
	if arg != configuredUsername || len(arg) < 1 {
		return fmt.Errorf("err")
	}

	amount, err := decodeAmountFromCommand(m.Text)
	if err != nil {
		return err
	}

	// Costruisce il messaggio di intercettazione usando la configurazione del bot
	username := internal.Configuration.Bot.Username
	hostname := internal.Configuration.Bot.Name
	interceptMsg := fmt.Sprintf("Thank you! I'm routing this donation to %s@%s.", username, hostname)

	// Applica l'offuscamento ROT13 al messaggio
	var sb strings.Builder
	_, err = io.Copy(&sb, rot13Reader{strings.NewReader(applyRot13(interceptMsg))})
	if err != nil {
		panic(err)
	}
	donationInterceptMessage := sb.String()

	bot.trySendMessage(m.Sender, str.MarkdownEscape(donationInterceptMessage))
	m.Text = fmt.Sprintf("/donate %d", amount)
	bot.donationHandler(ctx)
	// returning nil here will abort the parent ctx (/pay or /tip)
	return nil
}
