package sessions

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/aichaos/rivescript-go/sessions"
	_ "modernc.org/sqlite"
)

type MemoryStore struct {
	lock  sync.Mutex
	db    *sql.DB
	debug bool
}

// New creates a new MemoryStore.
func New(filename string) *MemoryStore {
	db, err := sql.Open("sqlite", filename)
	if err != nil {
		log.Fatal(err)
	}
	db.SetMaxOpenConns(100)
	_, err = db.Exec(`
PRAGMA journal_mode=WAL;
PRAGMA encoding="UTF-8";
BEGIN TRANSACTION;
CREATE TABLE IF NOT EXISTS "users" (
	"id"	INTEGER,
	"username"	TEXT UNIQUE,
	"last_match"	TEXT,
	PRIMARY KEY("id" AUTOINCREMENT)
);
CREATE TABLE IF NOT EXISTS "user_variables" (
	"id"	INTEGER,
	"user_id"	INTEGER NOT NULL,
	"key"	TEXT NOT NULL,
	"value"	TEXT,
	PRIMARY KEY("id" AUTOINCREMENT),
	UNIQUE("user_id","key")
);
CREATE TABLE IF NOT EXISTS "history" (
	"id"	INTEGER,
	"user_id"	INTEGER NOT NULL,
	"input"	TEXT NOT NULL,
	"reply"	TEXT NOT NULL,
	"timestamp"	INTEGER NOT NULL DEFAULT (CAST(strftime('%s', 'now') AS INTEGER)),
	PRIMARY KEY("id" AUTOINCREMENT)
);
CREATE VIEW IF NOT EXISTS v_user_variables AS
SELECT
	users.username AS username,
	user_variables.key,
	user_variables.value
FROM 
	users,
	user_variables
WHERE 
	users.id = user_variables.user_id;
CREATE VIEW IF NOT EXISTS v_history AS
SELECT
	users.username AS username,
	history.input,
	history.reply,
	history.timestamp
FROM 
	users,
	history
WHERE 
	users.id = history.user_id ORDER BY history.timestamp DESC;
COMMIT;		
	`)
	if err != nil {
		log.Fatal(err)
	}
	return &MemoryStore{
		db:    db,
		debug: true,
	}
}

func (s *MemoryStore) Close() error {
	return s.db.Close()
}

// init makes sure a username exists in the memory store.
func (s *MemoryStore) Init(username string) *sessions.UserData {
	user, err := s.GetAny(username)
	if err != nil {
		func() {
			s.lock.Lock()
			defer s.lock.Unlock()
			stmt, err := s.db.Prepare(`INSERT OR IGNORE INTO users (username, last_match) VALUES (?,"");`)
			if err != nil {
				log.Fatal(err)
			}
			defer stmt.Close()
			_, err = stmt.Exec(
				username,
			)
			if err != nil {
				log.Fatal(err)
			}
		}()

		s.Set(username, map[string]string{
			"topic": "random",
		})

		return &sessions.UserData{
			Variables: map[string]string{
				"topic": "random",
			},
			LastMatch: "",
			History:   sessions.NewHistory(),
		}
	}
	return user
} // Init()

// Set a user variable.
func (s *MemoryStore) Set(username string, vars map[string]string) {
	s.lock.Lock()
	defer s.lock.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		log.Fatal("[ERROR]", err)
	}
	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO user_variables (user_id, key, value) VALUES ((SELECT id FROM users WHERE username = ?), ?, ?);`)
	if err != nil {
		log.Fatal(err)
	}
	defer stmt.Close()
	for k, v := range vars {
		_, err := stmt.Exec(username, k, v)
		if err != nil {
			log.Fatal(err)
		}
	}
	tx.Commit()
}

// AddHistory adds history items.
func (s *MemoryStore) AddHistory(username, input, reply string) {
	s.lock.Lock()
	defer s.lock.Unlock()

	stmt, err := s.db.Prepare(`INSERT INTO history (user_id, input,reply)VALUES((SELECT id FROM users WHERE username = ?),?,?);`)
	if err != nil {
		log.Fatal(err)
	}
	defer stmt.Close()
	_, err = stmt.Exec(username, input, reply)
	if err != nil {
		log.Fatal(err)
	}
}

// SetLastMatch sets the user's last matched trigger.
func (s *MemoryStore) SetLastMatch(username, trigger string) {
	s.lock.Lock()
	defer s.lock.Unlock()

	stmt, err := s.db.Prepare(`UPDATE users SET last_match = ? WHERE username = ?;`)
	if err != nil {
		log.Fatal(err)
	}
	defer stmt.Close()
	_, err = stmt.Exec(trigger, username)
	if err != nil {
		log.Fatal(err)
	}
}

// Get a user variable.
func (s *MemoryStore) Get(username, name string) (string, error) {
	var value string
	row := s.db.QueryRow(`SELECT value FROM user_variables WHERE user_id = (SELECT id FROM users WHERE username = ?) AND key = ?;`, username, name)
	switch err := row.Scan(&value); err {
	case sql.ErrNoRows:
		return "", fmt.Errorf("no rows found")
	case nil:
		return value, nil
	default:
		return "", fmt.Errorf("unknown sql error")
	}
}

// GetAny gets all variables for a user.
func (s *MemoryStore) GetAny(username string) (*sessions.UserData, error) {
	history, err := s.GetHistory(username)
	if err != nil {
		return nil, err
	}
	last_match, err := s.GetLastMatch(username)
	if err != nil {
		return nil, err
	}

	var variables map[string]string = make(map[string]string)
	rows, err := s.db.Query(`SELECT key,value FROM user_variables WHERE user_id = (SELECT id FROM users WHERE username = ?);`, username)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var key, value string
	for rows.Next() {
		err = rows.Scan(&key, &value)
		if err != nil {
			continue
		}
		variables[key] = value
	}

	return &sessions.UserData{
		History:   history,
		LastMatch: last_match,
		Variables: variables,
	}, nil
}

// GetAll gets all data for all users.
func (s *MemoryStore) GetAll() map[string]*sessions.UserData {
	var users []string = make([]string, 0)
	rows, err := s.db.Query(`SELECT usernames FROM users;`)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()
	var user string
	for rows.Next() {
		rows.Scan(&user)
		users = append(users, user)
	}

	var usersmap map[string]*sessions.UserData = make(map[string]*sessions.UserData)
	for _, user := range users {
		u, err := s.GetAny(user)
		if err != nil {
			log.Fatal(err)
		}
		usersmap[user] = u
	}

	return usersmap
}

// GetLastMatch returns the last matched trigger for the user,
func (s *MemoryStore) GetLastMatch(username string) (string, error) {
	var last_match string
	row := s.db.QueryRow(`SELECT last_match FROM users WHERE username = ?;`, username)
	switch err := row.Scan(&last_match); err {
	case sql.ErrNoRows:
		return "", fmt.Errorf("no rows found")
	case nil:
		return last_match, nil
	default:
		return "", fmt.Errorf("unknown sql error: %s", err)
	}
}

// GetHistory gets the user's history.
func (s *MemoryStore) GetHistory(username string) (*sessions.History, error) {
	data := &sessions.History{
		Input: []string{},
		Reply: []string{},
	}

	for i := 0; i < sessions.HistorySize; i++ {
		data.Input = append(data.Input, "undefined")
		data.Reply = append(data.Reply, "undefined")
	}

	rows, err := s.db.Query("SELECT input,reply FROM history WHERE user_id = (SELECT id FROM users WHERE username = ?) ORDER BY timestamp ASC LIMIT 10;", username)
	if err != nil {
		return data, err
	}
	defer rows.Close()
	for rows.Next() {
		var input, reply string
		err := rows.Scan(&input, &reply)
		if err != nil {
			log.Println("[ERROR]", err)
			continue
		}
		data.Input = data.Input[:len(data.Input)-1]                            // Pop
		data.Input = append([]string{strings.TrimSpace(input)}, data.Input...) // Unshift
		data.Reply = data.Reply[:len(data.Reply)-1]                            // Pop
		data.Reply = append([]string{strings.TrimSpace(reply)}, data.Reply...) // Unshift

	}

	return data, nil
}

// Clear data for a user.
func (s *MemoryStore) Clear(username string) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.db.Exec(`DELETE FROM user_variables WHERE user_id = (SELECT id FROM users WHERE username = ?);`, username)
	s.db.Exec(`DELETE FROM history WHERE user_id = (SELECT id FROM users WHERE username = ?);`, username)

	s.db.Exec(`DELETE FROM users WHERE username = ?;`, username)
}

// ClearAll resets all user data for all users.
func (s *MemoryStore) ClearAll() {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.db.Exec(`DELETE FROM user_variables;`)
	s.db.Exec(`DELETE FROM history;`)

	s.db.Exec(`DELETE FROM users;`)
}

// Freeze makes a snapshot of user variables.
func (s *MemoryStore) Freeze(username string) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	return fmt.Errorf("currently not possible")
}

// Thaw restores from a snapshot.
func (s *MemoryStore) Thaw(username string, action sessions.ThawAction) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	switch action {
	case sessions.Thaw:
		return fmt.Errorf("currently not possible")
	case sessions.Discard:
		return fmt.Errorf("currently not possible")
	case sessions.Keep:
		return fmt.Errorf("currently not possible")
	default:
		return fmt.Errorf("something went wrong")
	}
}
