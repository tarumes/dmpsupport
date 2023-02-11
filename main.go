package main

import (
	"dmpsupport/rive"
	"flag"
	"fmt"
	"html/template"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
)

// https://discord.com/oauth2/authorize?client_id=1071810754623307926&scope=bot&permissions=117824

type Messages struct {
	ID      string
	Guild   string
	Channel string
	Author  string
	Content string
}

var messages []Messages

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

	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			templ, err := template.ParseFS(os.DirFS("./www/templates"), "*.html")
			if err != nil {
				log.Fatal(err)
			}

			switch r.Method {
			case http.MethodGet:
				templ.ExecuteTemplate(w, "index.html", messages)
			default:
				http.Error(
					w,
					http.StatusText(http.StatusMethodNotAllowed),
					http.StatusMethodNotAllowed,
				)
			}
		})
		mux.HandleFunc("/post", func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodPost:
				r.ParseForm()
				if r.FormValue("id") != "" && r.FormValue("guild") != "" && r.FormValue("channel") != "" && r.FormValue("content") != "" && r.FormValue("trigger") != "" {
					rs.LearnNew(r.FormValue("trigger"), r.FormValue("content"))
					messages = mmFilter(r.FormValue("id"))
					go func() {
						err = dg.ChannelTyping(r.FormValue("channel"))
						if err != nil {
							log.Printf("Couldn't start typing: %v", err)
						}
						time.Sleep(time.Second * 10)
						dg.ChannelMessageSendReply(r.FormValue("channel"), r.FormValue("content"), &discordgo.MessageReference{
							MessageID: r.FormValue("id"),
							GuildID:   r.FormValue("guild"),
							ChannelID: r.FormValue("channel"),
						})
					}()
				}
				http.Redirect(w, r, "/", http.StatusFound)
			default:
				http.Error(
					w,
					http.StatusText(http.StatusMethodNotAllowed),
					http.StatusMethodNotAllowed,
				)
			}
		})
		mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./www/static"))))
		srv := &http.Server{
			Handler:           mux,
			ReadTimeout:       time.Second * 15,
			WriteTimeout:      time.Second * 15,
			IdleTimeout:       time.Second * 15,
			ReadHeaderTimeout: time.Second * 15,
			Addr:              ":21616",
		}

		log.Fatal(srv.ListenAndServe())
	}()

	dg.AddHandler(func(s *discordgo.Session, m *discordgo.MessageUpdate) {
		if m.Author.ID == s.State.User.ID || !guilds[m.GuildID] || m.Content == "" {
			return
		}

		messages = mmFilter(m.ID)

		if reply, err := rs.Reply(m.Author.ID, m.Content); err != nil {
			messages = mmAdd(Messages{
				ID:      m.ID,
				Guild:   m.GuildID,
				Channel: m.ChannelID,
				Author:  m.Author.Username,
				Content: m.Content,
			})
			log.Println("[ERR]", err, m.Content)
		} else if reply != "" {
			log.Println("[INFO]", reply)

			defer s.ChannelTyping("")
			for i := 0; i < len(strings.Split(reply, "\n"))+int(RandomNumber(1, 6)); i = i + 1 {
				err = s.ChannelTyping(m.ChannelID)
				if err != nil {
					log.Printf("Couldn't start typing: %v", err)
				}
				time.Sleep(9900 * time.Millisecond)
			}

			if _, err := s.ChannelMessageSendReply(m.ChannelID, reply, m.Reference()); err != nil {
				log.Println("[ERR]", err)
			}
		}
	})

	dg.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		if m.Author.ID == s.State.User.ID || !guilds[m.GuildID] || m.Content == "" {
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

		if reply, err := rs.Reply(m.Author.ID, m.Content); err != nil {
			messages = mmAdd(Messages{
				ID:      m.ID,
				Guild:   m.GuildID,
				Channel: m.ChannelID,
				Author:  m.Author.Username,
				Content: m.Content,
			})
			log.Println("[ERR]", err, m.Content)
		} else if reply != "" {
			log.Println("[INFO]", reply)

			defer s.ChannelTyping("")
			for i := 0; i < len(strings.Split(reply, "\n"))+int(RandomNumber(1, 6)); i = i + 1 {
				err = s.ChannelTyping(m.ChannelID)
				if err != nil {
					log.Printf("Couldn't start typing: %v", err)
				}
				time.Sleep(9900 * time.Millisecond)
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

func RandomNumber(min, max int) int {
	return rand.Intn(max-min) + min
}

// webui messages
var mmlock sync.Mutex

func mmAdd(m Messages) []Messages {
	mmlock.Lock()
	defer mmlock.Unlock()

	mm := messages

	if len(mm) > 100 {
		mm = mm[1:]
	}

	mm = append(mm, m)
	return mm
}

func mmFilter(id string) []Messages {
	mmlock.Lock()
	defer mmlock.Unlock()

	var tmp []Messages = make([]Messages, 0)
	for _, v := range messages {
		if v.ID != id {
			tmp = append(tmp, v)
		}
	}
	return tmp
}
