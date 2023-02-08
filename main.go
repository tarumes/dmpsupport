package main

import (
	"dmpsupport/rive"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/bwmarrin/discordgo"
)

// https://discord.com/oauth2/authorize?client_id=1071810754623307926&scope=bot&permissions=117824

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "Debug mode, off by default")
	var token string
	flag.StringVar(&token, "token", "", "Discord Bot token.")
	flag.Parse()

	if token == "" {
		log.Fatal("no bot token defined, token is required")
	}

	rs := rive.New(debug)
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		log.Fatal(err)
	}

	dg.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		if m.Author.ID == s.State.User.ID {
			return
		}

		if reply, err := rs.Reply(m.Author.ID, m.Content); err != nil {
			log.Println(err, m.Content)
		} else if reply != "" {
			if _, err := s.ChannelMessageSendReply(m.ChannelID, reply, m.Reference()); err != nil {
				log.Println(err)
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
