package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"tg-up/version"

	"time"

	"github.com/amarnathcjd/gogram/telegram"
	"github.com/facette/natsort"
	"github.com/manifoldco/promptui"
	"gopkg.in/yaml.v2"
	_ "modernc.org/sqlite"
)

type VideoInfo struct {
	Title    string
	Filename string
	Duration float32
	Width    int
	Height   int
	Rotation int
}

type Config struct {
	APIID    int    `yaml:"api_id"`
	APIHash  string `yaml:"api_hash"`
	BotToken string `yaml:"bot_token"`
}

func printVersion() {
	fmt.Printf("tg-up version: %s\n", version.Version)
	fmt.Printf("Build: %s\n", version.BuildDate)
	fmt.Printf("Commit: %s\n", version.CommitHash)
}

func loadConfig(path string) Config {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("Failed to read config: %v", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		log.Fatalf("Failed to parse config: %v", err)
	}
	return cfg
}

func promptInput(label string) string {
	p := promptui.Prompt{Label: label}
	res, err := p.Run()
	if err != nil {
		log.Fatalf("Prompt failed: %v", err)
	}
	return res
}

// to do implement thumbnail generation later using ffmpeg

func GetTempThumbnailPath(suffix string) (string, error) {
	if suffix == "" {
		suffix = ".png"
	}

	f, err := os.CreateTemp("", "thumb-*"+suffix)
	if err != nil {
		return "", err
	}

	path := f.Name()
	f.Close()
	return path, nil
}

func promptChatID(db *sql.DB) string {
	_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS chat_history(chat_id TEXT PRIMARY KEY, chat_name TEXT)`)

	rows, err := db.Query("SELECT chat_id, chat_name FROM chat_history")
	if err != nil {
		log.Fatalf("Failed to query chat_history: %v", err)
	}
	defer rows.Close()

	type entry struct {
		ID   string
		Name string
	}

	historyEntries := []entry{}
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			continue
		}
		historyEntries = append(historyEntries, entry{ID: id, Name: name})
	}

	historyEntries = append(historyEntries, entry{ID: "new", Name: "Enter new chat ID"})

	// Build prompt items: "Chat Name (ID)" or just ID if no name
	items := []string{}
	for _, e := range historyEntries {
		if e.ID == "new" {
			items = append(items, e.Name)
		} else if e.Name != "" {
			items = append(items, e.Name+" ("+e.ID+")")
		} else {
			items = append(items, e.ID)
		}
	}

	sel := promptui.Select{Label: "Select chat", Items: items}
	idx, _, err := sel.Run()
	if err != nil {
		log.Fatalf("Prompt failed: %v", err)
	}

	selected := historyEntries[idx]
	if selected.ID == "new" {
		newID := promptInput("Enter channel/group ID (numeric or username)")
		chatName := promptInput("Enter chat name (optional, for history)")

		log.Printf("Stored chat name for history: %s", chatName)
		_, _ = db.Exec("INSERT OR IGNORE INTO chat_history(chat_id, chat_name) VALUES(?, ?)", newID, chatName)

		return newID
	}

	return selected.ID
}

func sortedEntries(folder string) []os.DirEntry {
	entries, _ := os.ReadDir(folder)

	names := []string{}
	entryMap := make(map[string]os.DirEntry)

	for _, e := range entries {
		// Skip hidden files
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}

		names = append(names, e.Name())
		entryMap[e.Name()] = e
	}

	// Natural sort
	// natsort.Sort(names)
	sort.Slice(names, func(i, j int) bool {
		// Compare lowercased versions of the filenames
		return natsort.Compare(strings.ToLower(names[i]), strings.ToLower(names[j]))
	})

	sorted := make([]os.DirEntry, 0, len(names))
	for _, n := range names {
		sorted = append(sorted, entryMap[n])
	}

	return sorted
}

func alreadyUploaded(db *sql.DB, path string) bool {
	row := db.QueryRow("SELECT 1 FROM uploaded WHERE path=?", path)
	var tmp int
	return row.Scan(&tmp) == nil
}

func markUploaded(db *sql.DB, path string) {
	_, _ = db.Exec("INSERT OR IGNORE INTO uploaded(path) VALUES(?)", path)
}

func uploadFile(client *telegram.Client, db *sql.DB, filePath, chatID string, replyToMessageID int32) {
	if alreadyUploaded(db, filePath) {
		fmt.Println("Skipping already uploaded:", filePath)
		return
	}
	fmt.Println("Uploading:", filePath)
	filename := filepath.Base(filePath)
	if replyToMessageID != 0 {
		_, err := client.SendMedia(chatID, filePath, &telegram.MediaOptions{Caption: filename, ReplyID: replyToMessageID})
		if err != nil {
			log.Printf("Failed upload %s: %v", filePath, err)
			return
		}
		return
	} else {
		_, err := client.SendMedia(chatID, filePath, &telegram.MediaOptions{Caption: filename})
		if err != nil {
			log.Printf("Failed upload %s: %v", filePath, err)
			return
		}
	}
	markUploaded(db, filePath)
}

func uploadFolder(client *telegram.Client, db *sql.DB, folder, chatID string, replyToMessageID int32) {
	entries := sortedEntries(folder)
	sleep := 2 * time.Second
	folderMsg, _ := client.SendMessage(chatID, "📁 "+filepath.Base(folder), &telegram.SendOptions{ReplyID: replyToMessageID})

	// handle files first
	for _, e := range entries {
		full := filepath.Join(folder, e.Name())
		if !e.IsDir() {
			time.Sleep(sleep)

			uploadFile(client, db, full, chatID, folderMsg.ID)
		}
	}

	// handle subfolders
	for _, e := range entries {
		full := filepath.Join(folder, e.Name())
		info, err := e.Info()
		if err != nil || !info.IsDir() {
			continue
		}
		fmt.Println("Entering folder:", full)
		time.Sleep(sleep)
		uploadFolder(client, db, full, chatID, folderMsg.ID)
	}
}

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		printVersion()
		return
	}
	cfgPath := flag.String("config", "", "Path to config YAML")
	flag.Parse()
	args := flag.Args()
	if *cfgPath == "" || len(args) < 1 {
		log.Fatal("Usage: tg-up --config /path/to/config.yml /path/to/data")
	}
	targetPath := args[0]

	cfg := loadConfig(*cfgPath)
	clientCfg := telegram.NewClientConfigBuilder(int32(cfg.APIID), cfg.APIHash).
		WithLogLevel(telegram.WarnLevel).Build()
	client, err := telegram.NewClient(clientCfg)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	client.Conn()
	if err := client.LoginBot(cfg.BotToken); err != nil {
		log.Fatalf("Bot login failed: %v", err)
	}
	botInfo, _ := client.GetMe()
	botName := botInfo.Username
	fmt.Printf("Logged in as bot: @%s\n", botName)
	dbPath := filepath.Join(filepath.Dir(*cfgPath), "tg-upload.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("Failed to open db: %v", err)
	}
	defer db.Close()
	_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS uploaded(path TEXT PRIMARY KEY,chat_id TEXT)`)

	chatID := promptChatID(db)
	// can't figure out how to use topics with gogram yet, so skipping that for now, even though topic id was passed in param
	// still sends to main chat
	// so using replyid as workaround

	replyId, err := strconv.Atoi(promptInput("Reply to message ID (0 for none)"))
	if err != nil {
		log.Fatalf("Invalid message ID: %v", err)
		replyId = 0
	}
	replyToMessageID := int32(replyId)
	info, err := os.Stat(targetPath)
	if err != nil {
		log.Fatalf("Failed to stat path: %v", err)
	}
	if info.IsDir() {
		uploadFolder(client, db, targetPath, chatID, replyToMessageID)
	} else {
		uploadFile(client, db, targetPath, chatID, replyToMessageID)
	}
	_, _ = client.SendMessage(chatID, "✅ All done", nil)

	fmt.Println("✅ All done")
}
