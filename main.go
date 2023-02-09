package main

import (
	"dmpsupport/rive"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
)

// https://discord.com/oauth2/authorize?client_id=1071810754623307926&scope=bot&permissions=117824

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "Debug mode, off by default")
	var token string
	flag.StringVar(&token, "token", "", "Discord Bot token.")
	var guild string
	flag.StringVar(&guild, "guild", "", "Discord Guilds to listen.")
	var admin string
	flag.StringVar(&admin, "admin", "", "Discord admins.")
	var geo string
	flag.StringVar(&geo, "geo", "", "token for geo api")
	flag.Parse()

	var replylock sync.Mutex

	var guilds map[string]bool = make(map[string]bool)
	for _, v := range strings.Split(guild, ",") {
		guilds[v] = true
	}
	var admins map[string]bool = make(map[string]bool)
	for _, v := range strings.Split(admin, ",") {
		admins[v] = true
	}
	if token == "" {
		log.Fatal("no bot token defined, token is required")
	}

	rs := rive.New(geo, debug)
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		log.Fatal(err)
	}

	dg.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		replylock.Lock()
		defer replylock.Unlock()

		if m.Author.ID == s.State.User.ID || !guilds[m.GuildID] {
			return
		}

		if strings.HasPrefix(m.Content, "!reload") && admins[m.Author.ID] {
			rs = rive.New(geo, debug)
			err = s.MessageReactionAdd(m.ChannelID, m.ID, "✅")
			if err != nil {
				log.Println("[ERR]", err)
				return
			}
			return
		}

		if strings.HasPrefix(m.Content, "!learn") && admins[m.Author.ID] {
			msg := strings.TrimSpace(strings.TrimPrefix(m.Content, "!learn"))
			g, err := getRegexGroup(`\x60{1,3}(\n|)(?P<trigger>\+\s\b[a-zA-Z ]{1,})\s(?P<reply>\-\s.*)\x60{1,3}`, msg)
			if err != nil {
				log.Println("[ERR]", err)
				return
			}
			new := fmt.Sprintf("\n%s\n%s\n",
				rs.GetUnicodePunctuation().ReplaceAllString(strings.ToLower(strings.TrimSpace(regexp.MustCompile(`\s{1,}`).ReplaceAllString(g["trigger"], " "))), ""),
				g["reply"],
			)
			err = rs.LearnNew(new)
			if err != nil {
				log.Println("[ERR]", err)
				return
			}
			err = writeBrain(new)
			if err != nil {
				log.Println("[ERR]", err)
				return
			}
			err = s.MessageReactionAdd(m.ChannelID, m.ID, "✅")
			if err != nil {
				log.Println("[ERR]", err)
				return
			}
			return
		}

		if reply, err := rs.Reply(m.Author.ID, m.Content); err != nil {
			log.Println("[ERR]", err, m.Content)
		} else if reply != "" {
			log.Println("[INFO]", reply)

			time.Sleep(time.Duration(
				RandomNumber(10000, 60000),
			) * time.Millisecond)

			defer s.ChannelTyping("")
			for i := 0; i < len(strings.Split(reply, "\n")); i = i + 1 {
				err = s.ChannelTyping(m.ChannelID)
				if err != nil {
					log.Printf("Couldn't start typing: %v", err)
				}
				time.Sleep(9433 * time.Millisecond)
			}

			if _, err := s.ChannelMessageSendReply(m.ChannelID, reply, m.Reference()); err != nil {
				log.Println("[ERR]", err)
			}
		}

	})

	dg.Identify.Intents = discordgo.IntentsGuildMessages

	if err = dg.Open(); err != nil {
		log.Fatal("error opening connection,", err)
	}

	// Wait here until CTRL-C or other term signal is received.
	fmt.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	// Cleanly close down the Discord session.
	if err := dg.Close(); err != nil {
		log.Fatal(err)
	}

	// Close Database connection
	if err := rs.Close(); err != nil {
		log.Fatal(err)
	}
}

func getRegexGroup(expr string, message string) (map[string]string, error) {
	var reply map[string]string = make(map[string]string)

	r, err := regexp.Compile(expr)
	if err != nil {
		return reply, err
	}

	data := r.FindStringSubmatch(message)
	if len(r.SubexpNames()) > 1 && len(data) == len(r.SubexpNames()) {

		for i, v := range r.SubexpNames() {
			if i != 0 && v != "" && data[i] != "" {
				reply[v] = data[i]
			}
		}
		return reply, nil
	}

	return reply, fmt.Errorf("no index groups found %s", expr)
}

var flock sync.Mutex

func writeBrain(in string) error {
	flock.Lock()
	defer flock.Unlock()

	f, err := os.OpenFile("brain.rive", os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(in)
	if err != nil {
		return err
	}

	return nil
}

func RandomNumber(min, max int) int {
	return rand.Intn(max-min) + min
}
