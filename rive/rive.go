package rive

import (
	"dmpsupport/rive/geoapi"
	geohelpers "dmpsupport/rive/geoapi/helpers"
	"dmpsupport/rive/sessions"
	"fmt"
	"math"
	"strconv"

	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/aichaos/rivescript-go"
	"github.com/aichaos/rivescript-go/lang/javascript"
)

type Client struct {
	r       *rivescript.RiveScript
	session *sessions.MemoryStore
	debug   bool
	log     map[string][]Message
	geo     *geoapi.Client

	lock sync.Mutex
}

type Message struct {
	Content string
	Time    time.Time
}

var spaces *regexp.Regexp = regexp.MustCompile(`\s{1,}`)

func New(geotoken string, debug bool) *Client {
	var session *sessions.MemoryStore = sessions.New("sessions.db")
	geo := geoapi.New(geotoken)
	r := rivescript.New(&rivescript.Config{
		Debug:          debug,                 // Debug mode, off by default
		Strict:         true,                  // Strict syntax checking
		UTF8:           true,                  // UTF-8 support enabled by default
		Depth:          50,                    // Becomes default 50 if Depth is <= 0
		Seed:           time.Now().UnixNano(), // Random number seed (default is == 0)
		SessionManager: session,               // Default in-memory session manager
	})
	r.SetHandler("javascript", javascript.New(r))
	if err := r.LoadFile("brain.rive"); err != nil {
		return nil
	}
	if err := r.SortReplies(); err != nil {
		return nil
	}

	// Subroutines
	{
		r.SetSubroutine("gpsdistance", func(rs *rivescript.RiveScript, s []string) string {
			pos1, err := geo.GetLocationCache(s[0])
			if err != nil {
				return "undefined" // fmt.Sprintf("[ERROR] %s %s ", err.Error(), strings.Join(s, ","))
			}
			pos2, err := geo.GetLocationCache(s[1])
			if err != nil {
				return "undefined" //fmt.Sprintf("[ERROR] %s %s ", err.Error(), strings.Join(s, ","))
			}
			return fmt.Sprintf("%.2f", geohelpers.GPSDistance(pos1, pos2))
		})
		r.SetSubroutine("percent", func(rs *rivescript.RiveScript, s []string) string {
			percent := func(part float64, total float64) float64 {
				return (float64(part) * float64(100)) / float64(total)
			}

			const emp string = "▱"
			const ful string = "▰"

			part, err := strconv.ParseFloat(s[0], 64)
			if err != nil {
				return strings.Repeat(emp, 10)
			}

			total, err := strconv.ParseFloat(s[1], 64)
			if err != nil {
				return strings.Repeat(emp, 10)
			}

			if part > total {
				return fmt.Sprintf("%s %.2f%%", strings.Repeat(ful, 10), 100.00)
			}
			per := percent(part, total)
			p := int(math.RoundToEven(per / 10))

			return fmt.Sprintf("%s%s %.2f%%", strings.Repeat(ful, p), strings.Repeat(emp, 10-p), per)
		})
	}

	return &Client{
		r:       r,
		session: session,
		debug:   debug,
		geo:     geo,
		log:     make(map[string][]Message),
	}
}

func (c *Client) Close() error {
	c.geo.Close()
	return c.session.Close()
}

func (c *Client) GetUnicodePunctuation() *regexp.Regexp {
	return c.r.UnicodePunctuation
}

func (c *Client) LearnNew(in string) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	err := c.r.Stream(in)
	if err != nil {
		return err
	}
	return c.r.SortReplies()
}

func (c *Client) Reply(username, message string) (string, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	message = strings.TrimSpace(spaces.ReplaceAllString(message, " "))

	c.log[username] = append(c.log[username], Message{
		Content: message,
		Time:    time.Now(),
	})
	c.log[username] = func(in []Message) []Message {
		var reply []Message = make([]Message, 0)
		for _, v := range in {
			if time.Since(v.Time) < time.Second*30 {
				reply = append(reply, v)
			}
		}
		return reply
	}(c.log[username])

	if r, err := c.r.Reply(username, message); err != nil {
		var tmp string
		for i := len(c.log[username]) - 1; i >= 0; i-- {
			tmp = strings.TrimSpace(spaces.ReplaceAllString(fmt.Sprintf("%s %s", c.log[username][i].Content, tmp), " "))
			if r, err := c.r.Reply(username, tmp); err == nil {
				c.log[username] = make([]Message, 0)
				return r, nil
			}
		}
		return "", fmt.Errorf("no trigger matched")
	} else {
		return r, nil
	}
}
