package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type DB struct {
	*sql.DB
}

func OpenDB(path string) (*DB, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		return nil, err
	}
	return &DB{db}, nil
}

func (d *DB) Migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS categories (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			key TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			icon TEXT NOT NULL DEFAULT 'generic',
			color TEXT NOT NULL DEFAULT '#888888',
			fields TEXT NOT NULL DEFAULT '[]',
			relations TEXT NOT NULL DEFAULT '[]',
			sort INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS items (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			category_id INTEGER NOT NULL REFERENCES categories(id),
			name TEXT NOT NULL,
			brand TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'collecting',
			purchase_date TEXT NOT NULL DEFAULT '',
			purchase_price REAL NOT NULL DEFAULT 0,
			parted_date TEXT NOT NULL DEFAULT '',
			parted_price REAL,
			note TEXT NOT NULL DEFAULT '',
			fields TEXT NOT NULL DEFAULT '{}',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS images (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			item_id INTEGER NOT NULL REFERENCES items(id) ON DELETE CASCADE,
			filename TEXT NOT NULL,
			thumb TEXT NOT NULL,
			sort INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS item_relations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			item_id INTEGER NOT NULL REFERENCES items(id) ON DELETE CASCADE,
			related_item_id INTEGER NOT NULL REFERENCES items(id) ON DELETE CASCADE,
			UNIQUE(item_id, related_item_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_items_category ON items(category_id)`,
		`CREATE INDEX IF NOT EXISTS idx_items_status ON items(status)`,
		`CREATE INDEX IF NOT EXISTS idx_images_item ON images(item_id)`,
		`CREATE INDEX IF NOT EXISTS idx_relations_item ON item_relations(item_id)`,
		`CREATE INDEX IF NOT EXISTS idx_relations_related ON item_relations(related_item_id)`,
	}
	for _, s := range stmts {
		if _, err := d.Exec(s); err != nil {
			return err
		}
	}
	return nil
}

type seedCategory struct {
	Key       string
	Name      string
	Icon      string
	Color     string
	Fields    []FieldDef
	Relations []RelationDef
	Sort      int
}

func (d *DB) Seed() error {
	seeds := []seedCategory{
		{
			Key: "pen", Name: "钢笔", Icon: "pen", Color: "#3a4a5a", Sort: 1,
			Fields:    []FieldDef{{Key: "nib", Label: "笔尖类型", Type: "text"}},
			Relations: []RelationDef{{TargetKey: "ink", Label: "搭配墨水"}},
		},
		{
			Key: "ink", Name: "墨水", Icon: "ink", Color: "#2b3a67", Sort: 2,
			Fields:    []FieldDef{{Key: "color", Label: "颜色", Type: "text"}},
			Relations: []RelationDef{{TargetKey: "pen", Label: "搭配钢笔"}},
		},
		{
			Key: "inkstone", Name: "砚台", Icon: "inkstone", Color: "#5c4a3a", Sort: 3,
			Fields: []FieldDef{{Key: "pit", Label: "坑口", Type: "text"}},
		},
		{
			Key: "inkstick", Name: "墨条", Icon: "inkstick", Color: "#3c3c40", Sort: 4,
			Fields: []FieldDef{
				{Key: "era", Label: "年代", Type: "text"},
				{Key: "type", Label: "类型", Type: "text"},
			},
		},
	}
	for _, s := range seeds {
		var count int
		if err := d.QueryRow(`SELECT COUNT(*) FROM categories WHERE key = ?`, s.Key).Scan(&count); err != nil {
			return err
		}
		if count > 0 {
			continue
		}
		fields, err := json.Marshal(s.Fields)
		if err != nil {
			return err
		}
		relations := []byte("[]")
		if s.Relations != nil {
			relations, err = json.Marshal(s.Relations)
			if err != nil {
				return err
			}
		}
		_, err = d.Exec(
			`INSERT INTO categories (key, name, icon, color, fields, relations, sort, created_at) VALUES (?,?,?,?,?,?,?,?)`,
			s.Key, s.Name, s.Icon, s.Color, string(fields), string(relations), s.Sort, now(),
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func now() string {
	return time.Now().Format("2006-01-02 15:04:05")
}
