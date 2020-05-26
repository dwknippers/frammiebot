package main

import (
	"log"
	"github.com/gempir/go-twitch-irc/v2"
	"os"
	"regexp"
	"time"
)
const VERSION = "1.1"

// The introduction message shown when the bot joins a channel and
// on standard output.
const INTRODUCTION = "frammiebot v"+VERSION+" IN DA HOOS 4Head KEKW ."

// The OAuth token to use for authorization to a Twitch channel.
const ENV_TOKEN = "TWITCH_OAUTH_TOKEN"

var client *twitch.Client

// Collection of various compiled regular expressions.
var regex = map[string]*regexp.Regexp {
	"command": regexp.MustCompile(`^\!(.*)$`),
	"message": regexp.MustCompile(`(\w|\:)+`),
	"water": regexp.MustCompile(`(?i)(w[a|ā]t[e|ē]r)`),
}

// A BettingRound is a single round of betting on a channel.
type BettingRound struct {
	closed bool
	bets map[string][]time.Time
}

// The currently open betting rounds per channel.
var channelBets = make(map[string]*BettingRound)

// Whether or not the given user is allowed to perform a task that requires
// additional permissions.
func authorized(user *twitch.User) bool {
	return user.Badges["broadcaster"] + user.Badges["moderator"] > 0 || user.Name == "frammie"
}

// Used to respond to incoming messages using a standard form.
func respond(message *twitch.PrivateMessage, response string) {
	client.Say(message.Channel, message.User.DisplayName + " -> " + response)
}

// Converts given input array of strings to array of time.Time, or if failed,
// notify the requester and return error.
func formatTimes(times []string, message *twitch.PrivateMessage) ([]time.Time, error) {
	ft := make([]time.Time, len(times))
	for i, t := range times {
		pt, err := time.Parse("15:04", t)
		if err != nil {
			respond(message, "Could not read your time(s).")
			return nil, err
		} else {
			ft[i] = pt
		}
	}
	return ft, nil
}

// Checks if on the channel the message originated from there is currently
// a bidding round going on.
func checkActiveBidding(message *twitch.PrivateMessage) bool {
	if _, exist := channelBets[message.Channel]; exist == false {
		respond(message, "There is currently no active bidding!")
		return false
	}
	return true
}

// Primary message event handler used for parsing commands related to all
// fields of operation of this bot.
func onPrivateMessage(message twitch.PrivateMessage) {

	if regex["water"].MatchString(message.Message) {
		client.Say(message.Channel, "☕☕ Coffee is better! peepoCoffee ")
	}

	split := regex["command"].FindStringSubmatch(message.Message)
	if len(split) > 1 {
		parts := regex["message"].FindAllString(split[1], -1)

		switch parts[0] {
			case "bet":
				if len(parts) < 2 { return }

				switch parts[1] {
					// Starts a new betting round
					case "start":
						if !authorized(&message.User) { return }
						client.Say(message.Channel, "Betting has started! Place your bets below!")
						channelBets[message.Channel] = &BettingRound{bets: make(map[string][]time.Time)}
					// Closes an existing betting round
					case "close":
						if !authorized(&message.User) { return }
						if !checkActiveBidding(&message) { return }
						channelBets[message.Channel].closed = true
						client.Say(message.Channel, "Betting has closed! Everyone, good luck!")
					// Ends a betting round
					case "end":
						if !authorized(&message.User) { return }
						if !checkActiveBidding(&message) { return }
						if len(parts) < 3 {
							respond(&message, "Format: bet end [time...]")
							return
						}

						results, err := formatTimes(parts[2:], &message)
						if err != nil { return }

						// Determine winners
						winners := make([]string, 0, 5)
						determine:
						for user, times := range channelBets[message.Channel].bets {
							for i := 0; i < len(results); i++ {
								if i > len(times)-1 || !times[i].Equal(results[i]) {
									continue determine
								}
							}
							// Winner
							winners = append(winners, user)
						}

						if len(winners) > 0 {
							winMessage := "🎉 Congratulations to following winner(s): "
							for _, winner := range winners {
								winMessage += "🥳 - " + winner + " "
							}
							client.Say(message.Channel, winMessage)
						} else {
							client.Say(message.Channel, "✨ Unfortunately no winners this time, good luck on the next betting round!")
						}

						delete(channelBets, message.Channel)
					// By default, handle !bet prefix messages as actual bets.
					default:
						if !checkActiveBidding(&message) { return }
						if channelBets[message.Channel].closed { return }

						times, err := formatTimes(parts[1:], &message)
						if err != nil { return }

						// Record/update bet
						channelBets[message.Channel].bets[message.User.DisplayName] = times
						log.Println(message.User.DisplayName + " betted")
				}
		}
	}
}

func main() {
	// Retrieve OAuth token from operating system environment.
	token, exist := os.LookupEnv(ENV_TOKEN)
	if !exist {
		log.Fatal("Failed to find token in environment variable "+ENV_TOKEN)
	}

	client = twitch.NewClient("frammiebot", "oauth:"+token)

	// Validate arguments.
	if len(os.Args) < 2 {
		log.Fatal("No channels to join specified. Format: frammiebot [channel...]")
	}

	log.Println(INTRODUCTION)

	// Join channel names as given as arguments.
	for _, channel := range os.Args[1:] {
		client.Join(channel)
		client.Say(channel, INTRODUCTION)
	}

	// Register handlers
	client.OnPrivateMessage(onPrivateMessage);

	err := client.Connect()
	if err != nil {
		log.Fatal(err.Error())
	}
}
