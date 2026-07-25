package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func scanCategory(row interface{ Scan(...any) error }) (*Category, error) {
	var c Category
	var fields, relations string
	if err := row.Scan(&c.ID, &c.Key, &c.Name, &c.Icon, &c.Color, &fields, &relations, &c.Sort, &c.CreatedAt); err != nil {
		return nil, err
	}
	c.Fields = []FieldDef{}
	c.Relations = []RelationDef{}
	if fields != "" {
		_ = json.Unmarshal([]byte(fields), &c.Fields)
	}
	if relations != "" {
		_ = json.Unmarshal([]byte(relations), &c.Relations)
	}
	if c.Fields == nil {
		c.Fields = []FieldDef{}
	}
	if c.Relations == nil {
		c.Relations = []RelationDef{}
	}
	return &c, nil
}

const categoryCols = `id, key, name, icon, color, fields, relations, sort, created_at`

func (s *Server) getCategoryByID(id int64) (*Category, error) {
	row := s.db.QueryRow(`SELECT `+categoryCols+` FROM categories WHERE id = ?`, id)
	c, err := scanCategory(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return c, err
}

func (s *Server) listCategories(c *gin.Context) {
	rows, err := s.db.Query(`SELECT ` + categoryCols + ` FROM categories ORDER BY sort, id`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	list := []*Category{}
	for rows.Next() {
		cat, err := scanCategory(rows)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		list = append(list, cat)
	}
	for _, cat := range list {
		_ = s.db.QueryRow(`SELECT COUNT(*) FROM items WHERE category_id = ?`, cat.ID).Scan(&cat.ItemCount)
	}
	c.JSON(http.StatusOK, list)
}

type categoryInput struct {
	Key       string        `json:"key"`
	Name      string        `json:"name"`
	Icon      string        `json:"icon"`
	Color     string        `json:"color"`
	Fields    []FieldDef    `json:"fields"`
	Relations []RelationDef `json:"relations"`
	Sort      int           `json:"sort"`
}

func (in *categoryInput) validate() (string, []byte, []byte) {
	if in.Key == "" {
		return "key 不能为空", nil, nil
	}
	if in.Name == "" {
		return "name 不能为空", nil, nil
	}
	if in.Icon == "" {
		in.Icon = "generic"
	}
	if in.Color == "" {
		in.Color = "#888888"
	}
	if in.Fields == nil {
		in.Fields = []FieldDef{}
	}
	if in.Relations == nil {
		in.Relations = []RelationDef{}
	}
	fields, err := json.Marshal(in.Fields)
	if err != nil {
		return "fields 格式错误", nil, nil
	}
	relations, err := json.Marshal(in.Relations)
	if err != nil {
		return "relations 格式错误", nil, nil
	}
	return "", fields, relations
}

func (s *Server) createCategory(c *gin.Context) {
	var in categoryInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误: " + err.Error()})
		return
	}
	msg, fields, relations := in.validate()
	if msg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}
	res, err := s.db.Exec(
		`INSERT INTO categories (key, name, icon, color, fields, relations, sort, created_at) VALUES (?,?,?,?,?,?,?,?)`,
		in.Key, in.Name, in.Icon, in.Color, string(fields), string(relations), in.Sort, now(),
	)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "创建失败（key 可能已存在）: " + err.Error()})
		return
	}
	id, _ := res.LastInsertId()
	cat, _ := s.getCategoryByID(id)
	c.JSON(http.StatusCreated, cat)
}

func (s *Server) updateCategory(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	existing, err := s.getCategoryByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "品类不存在"})
		return
	}
	var in categoryInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误: " + err.Error()})
		return
	}
	msg, fields, relations := in.validate()
	if msg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}
	_, err = s.db.Exec(
		`UPDATE categories SET key=?, name=?, icon=?, color=?, fields=?, relations=?, sort=? WHERE id=?`,
		in.Key, in.Name, in.Icon, in.Color, string(fields), string(relations), in.Sort, id,
	)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "更新失败（key 可能已存在）: " + err.Error()})
		return
	}
	cat, _ := s.getCategoryByID(id)
	c.JSON(http.StatusOK, cat)
}

func (s *Server) deleteCategory(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM items WHERE category_id = ?`, id).Scan(&count); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "该品类下还有藏品，无法删除"})
		return
	}
	res, err := s.db.Exec(`DELETE FROM categories WHERE id = ?`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "品类不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
