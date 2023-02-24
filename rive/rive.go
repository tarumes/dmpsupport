package rive

import (
	"database/sql"
	"dmpsupport/rive/geoapi"
	geohelpers "dmpsupport/rive/geoapi/helpers"
	"dmpsupport/rive/sessions"
	"fmt"
	"log"
	"math"
	"strconv"

	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/aichaos/rivescript-go"
	// "github.com/aichaos/rivescript-go/lang/javascript"
	"dmpsupport/rive/handlers/javascript"
)

type Client struct {
	r       *rivescript.RiveScript
	session *sessions.MemoryStore

	db *sql.DB

	debug bool
	geo   *geoapi.Client

	lock sync.Mutex
}

type Message struct {
	Content string
	Time    time.Time
}

var spaces *regexp.Regexp = regexp.MustCompile(`\s{1,}`)

func New(debug bool) *Client {
	var session *sessions.MemoryStore = sessions.New("session.db")
	geo := geoapi.New()

	db, err := sql.Open("sqlite", "rivescript.db")
	if err != nil {
		log.Fatal(err)
	}
	_, err = db.Exec(`
	PRAGMA journal_mode = 'WAL';
	BEGIN TRANSACTION;
	CREATE TABLE IF NOT EXISTS "learned" (
		"trigger"	TEXT NOT NULL,
		"reply"	TEXT NOT NULL
	);
	COMMIT;`)
	if err != nil {
		log.Fatal(err)
	}

	r := rivescript.New(&rivescript.Config{
		Debug:          debug,                 // Debug mode, off by default
		Strict:         true,                  // Strict syntax checking
		UTF8:           true,                  // UTF-8 support enabled by default
		Depth:          50,                    // Becomes default 50 if Depth is <= 0
		Seed:           time.Now().UnixNano(), // Random number seed (default is == 0)
		SessionManager: session,               // Default in-memory session manager
	})
	r.SetUnicodePunctuation(`[.,!?;:"@]`)
	r.SetHandler("javascript", javascript.New(r))
	if err := r.LoadDirectory("brain"); err != nil {
		return nil
	}

	var l []string = make([]string, 0)
	rows, err := db.Query(`SELECT DISTINCT trigger, reply FROM learned;`)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()
	var trigger, reply string
	for rows.Next() {
		if err := rows.Scan(&trigger, &reply); err == nil {
			l = append(l, fmt.Sprintf("+ %s\n- %s\n", trigger, reply))
		}
	}
	err = r.Stream(strings.Join(l, "\n"))
	if err != nil {
		log.Fatal(err)
	}
	if err := r.SortReplies(); err != nil {
		return nil
	}

	// Subroutines
	{
		r.SetSubroutine("since", func(rs *rivescript.RiveScript, s []string) string {
			t1, err := time.Parse(time.RFC3339, s[0])
			if err != nil {
				fmt.Println(err)
				return "{{ERROR}}"
			}
			t2, err := time.Parse(time.RFC3339, s[1])
			if err != nil {
				fmt.Println(err)
				return "{{ERROR}}"
			}

			if t1.Before(t2) {
				return fmt.Sprint(t1.Sub(t2).Seconds())
			} else {
				return fmt.Sprint(t2.Sub(t1))
			}
		})
		r.SetSubroutine("gpsdistance", func(rs *rivescript.RiveScript, s []string) string {
			pos1, err := geo.GetLocation(s[0])
			if err != nil {
				return "undefined" // fmt.Sprintf("[ERROR] %s %s ", err.Error(), strings.Join(s, ","))
			}
			pos2, err := geo.GetLocation(s[1])
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
		db:      db,
		debug:   debug,
		geo:     geo,
	}
}

func (c *Client) Close() error {
	c.geo.Close()
	return c.session.Close()
}

func (c *Client) GetUnicodePunctuation() *regexp.Regexp {
	return c.r.UnicodePunctuation
}

func (c *Client) LearnNew(trigger string, reply string) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	trigger = strings.TrimSpace(spaces.ReplaceAllString(c.r.UnicodePunctuation.ReplaceAllString(strings.ToLower(trigger), ""), " "))

	stmt, err := c.db.Prepare(`INSERT INTO learned (trigger, reply)VALUES(?,?);`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	_, err = stmt.Exec(trigger, reply)
	if err != nil {
		return err
	}

	err = c.r.Stream(fmt.Sprintf("+ %s\n- %s\n\n", trigger, reply))
	if err != nil {
		return err
	}
	return c.r.SortReplies()
}

func (c *Client) Reply(username, message string) (string, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	msg := strings.TrimSpace(spaces.ReplaceAllString(message, " "))

	var pf string = c.r.UnicodePunctuation.String()
	c.r.SetUnicodePunctuation(``)
	defer c.r.SetUnicodePunctuation(pf)

	if r, err := c.r.Reply(username, msg); err != nil {
		c.r.SetUnicodePunctuation(pf)
		if r2, err := c.r.Reply(username, msg); err != nil {
			log.Println(err,c.r.UnicodePunctuation.ReplaceAllString(msg, ""))
			return "", err
		} else if r2 == "" {
			return "", fmt.Errorf("empty reply")
		} else {
			return r2, nil
		}
	} else if r == "" {
		return "", fmt.Errorf("empty reply")
	} else {
		return r, nil
	}
}
