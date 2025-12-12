package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"tg-up/version"
	"time"

	"github.com/amarnathcjd/gogram/telegram"
	"github.com/facette/natsort"
	"github.com/manifoldco/promptui"
	"gopkg.in/yaml.v2"
	_ "modernc.org/sqlite"
)

type Config struct {
	APIID    int    `yaml:"api_id"`
	APIHash  string `yaml:"api_hash"`
	BotToken string `yaml:"bot_token"`
}

func printVersion() {
	fmt.Printf("tg-up version: %s\n", version.Version)
	fmt.Printf("Build: %s\n", version.BuildDate)
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

func promptChatID(db *sql.DB) string {
	_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS chat_history(chat_id TEXT PRIMARY KEY)`)
	rows, err := db.Query("SELECT chat_id FROM chat_history")
	if err != nil {
		log.Fatalf("Failed to query chat_history: %v", err)
	}
	defer rows.Close()

	history := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}
		history = append(history, id)
	}
	history = append(history, "Enter new chat ID")

	sel := promptui.Select{Label: "Select chat ID", Items: history}
	idx, _, err := sel.Run()
	if err != nil {
		log.Fatalf("Prompt failed: %v", err)
	}

	if history[idx] == "Enter new chat ID" {
		newID := promptInput("Enter channel/group ID (numeric or username)")
		_, _ = db.Exec("INSERT OR IGNORE INTO chat_history(chat_id) VALUES(?)", newID)
		return newID
	}
	return history[idx]
}

func sortedEntries(folder string) []os.DirEntry {
	entries, _ := os.ReadDir(folder)
	names := make([]string, len(entries))
	entryMap := make(map[string]os.DirEntry)
	for i, e := range entries {
		// Skip hidden files and folders
		if e.Name()[0] == '.' {
			continue
		}
		names[i] = e.Name()
		entryMap[e.Name()] = e
	}
	natsort.Sort(names)
	sorted := make([]os.DirEntry, 0, len(names))
	for _, n := range names {
		if e, ok := entryMap[n]; ok {
			sorted = append(sorted, e)
		}
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

func uploadFile(client *telegram.Client, db *sql.DB, filePath, chatID string) {
	if alreadyUploaded(db, filePath) {
		fmt.Println("Skipping already uploaded:", filePath)
		return
	}
	fmt.Println("Uploading:", filePath)
	_, err := client.SendMedia(chatID, filePath, &telegram.MediaOptions{Caption: ""})
	if err != nil {
		log.Printf("Failed upload %s: %v", filePath, err)
		return
	}
	markUploaded(db, filePath)
}

func uploadFolder(client *telegram.Client, db *sql.DB, folder, chatID string) {
	entries := sortedEntries(folder)
	sleep := 2 * time.Second

	// handle files first
	for _, e := range entries {
		full := filepath.Join(folder, e.Name())
		if !e.IsDir() {
			time.Sleep(sleep)
			uploadFile(client, db, full, chatID)
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
		uploadFolder(client, db, full, chatID)
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

	dbPath := filepath.Join(filepath.Dir(*cfgPath), "tg-upload.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("Failed to open db: %v", err)
	}
	defer db.Close()
	_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS uploaded(path TEXT PRIMARY KEY)`)

	chatID := promptChatID(db)

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

	_, _ = client.SendMessage(filepath.Base(targetPath), chatID, nil)

	info, err := os.Stat(targetPath)
	if err != nil {
		log.Fatalf("Failed to stat path: %v", err)
	}
	if info.IsDir() {
		uploadFolder(client, db, targetPath, chatID)
	} else {
		uploadFile(client, db, targetPath, chatID)
	}

	fmt.Println("✅ All done")
}
