package main

import "encoding/json"

type FieldDef struct {
	Key     string   `json:"key"`
	Label   string   `json:"label"`
	Type    string   `json:"type"` // text | select
	Options []string `json:"options,omitempty"`
	// Filterable 表示该字段是否在品类列表页作为筛选项；nil 视为可筛选（兼容旧数据）
	Filterable *bool `json:"filterable,omitempty"`
}

type RelationDef struct {
	TargetKey string `json:"target_key"`
	Label     string `json:"label"`
}

type Category struct {
	ID        int64         `json:"id"`
	Key       string        `json:"key"`
	Name      string        `json:"name"`
	Icon      string        `json:"icon"`
	Color     string        `json:"color"`
	Fields    []FieldDef    `json:"fields"`
	Relations []RelationDef `json:"relations"`
	Sort      int           `json:"sort"`
	CreatedAt string        `json:"created_at"`
	ItemCount int           `json:"item_count"`
}

type Item struct {
	ID            int64           `json:"id"`
	CategoryID    int64           `json:"category_id"`
	Name          string          `json:"name"`
	Brand         string          `json:"brand"`
	Status        string          `json:"status"`
	PurchaseDate  string          `json:"purchase_date"`
	PurchasePrice float64         `json:"purchase_price"`
	PartedDate    string          `json:"parted_date"`
	PartedPrice   *float64        `json:"parted_price"`
	Note          string          `json:"note"`
	Fields        json.RawMessage `json:"fields"`
	CreatedAt     string          `json:"created_at"`
	UpdatedAt     string          `json:"updated_at"`
}

type Image struct {
	ID       int64  `json:"id"`
	ItemID   int64  `json:"item_id"`
	Filename string `json:"filename"`
	Thumb    string `json:"thumb"`
	Sort     int    `json:"sort"`
}

const (
	StatusCollecting = "collecting"
	StatusParted     = "parted"
)
