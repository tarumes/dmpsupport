package rive

import (
	"dmpsupport/rive/sessions"
	"fmt"
	"log"

	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/aichaos/rivescript-go"
	"github.com/aichaos/rivescript-go/lang/javascript"
)

type Client struct {
	r *rivescript.RiveScript
	s *sessions.MemoryStore
	d bool
	l map[string][]Message

	lock sync.Mutex
}

type Message struct {
	Content string
	Time    time.Time
}

var spaces *regexp.Regexp = regexp.MustCompile(`\s{1,}`)

func New(debug bool) *Client {
	var s *sessions.MemoryStore = sessions.New("sessions.db")
	r := rivescript.New(&rivescript.Config{
		Debug:          debug,                 // Debug mode, off by default
		Strict:         true,                  // Strict syntax checking
		UTF8:           true,                  // UTF-8 support enabled by default
		Depth:          50,                    // Becomes default 50 if Depth is <= 0
		Seed:           time.Now().UnixNano(), // Random number seed (default is == 0)
		SessionManager: s,                     // Default in-memory session manager
	})
	r.SetUnicodePunctuation(``)
	r.SetHandler("javascript", javascript.New(r))

	if err := r.LoadFile("brain.rive"); err != nil {
		return nil
	}
	if err := r.SortReplies(); err != nil {
		return nil
	}

	return &Client{
		r: r,
		s: s,
		d: debug,
		l: make(map[string][]Message),
	}
}

func (c *Client) Close() error {
	return c.s.Close()
}

func (c *Client) Reply(username, message string) (string, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	message = strings.TrimSpace(spaces.ReplaceAllString(message, " "))

	c.l[username] = append(c.l[username], Message{
		Content: message,
		Time:    time.Now(),
	})
	c.l[username] = func(in []Message) []Message {
		var reply []Message = make([]Message, 0)
		for _, v := range in {
			if time.Since(v.Time) < time.Second*30 {
				reply = append(reply, v)
			}
		}
		return reply
	}(c.l[username])

	if r, err := c.r.Reply(username, message); err != nil {
		var tmp string
		for i := len(c.l[username]) - 1; i >= 0; i-- {
			tmp = strings.TrimSpace(spaces.ReplaceAllString(fmt.Sprintf("%s %s", c.l[username][i].Content, tmp), " "))
			fmt.Printf("|%s|\n", tmp)
			if r, err := c.r.Reply(username, tmp); err == nil {
				log.Println(tmp, r)
				return r, nil
			}
		}
		return "", fmt.Errorf("no trigger matched")
	} else {
		return r, nil
	}
}
