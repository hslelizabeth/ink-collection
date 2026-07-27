package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

var fieldKeyRe = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

type itemPayload struct {
	CategoryID    int64          `json:"category_id"`
	Name          string         `json:"name"`
	Brand         string         `json:"brand"`
	Status        string         `json:"status"`
	PurchaseDate  string         `json:"purchase_date"`
	PurchasePrice float64        `json:"purchase_price"`
	PartedDate    string         `json:"parted_date"`
	PartedPrice   *float64       `json:"parted_price"`
	Note          string         `json:"note"`
	Fields        map[string]any `json:"fields"`
	RelatedIDs    []int64        `json:"related_ids"`
}

func (in *itemPayload) validate() (string, []byte) {
	if in.CategoryID == 0 {
		return "category_id 不能为空", nil
	}
	if strings.TrimSpace(in.Name) == "" {
		return "name 不能为空", nil
	}
	if in.Status == "" {
		in.Status = StatusCollecting
	}
	if in.Status != StatusCollecting && in.Status != StatusParted {
		return "status 只能是 collecting 或 parted", nil
	}
	if in.Fields == nil {
		in.Fields = map[string]any{}
	}
	fields, err := json.Marshal(in.Fields)
	if err != nil {
		return "fields 格式错误", nil
	}
	return "", fields
}

func scanItem(row interface{ Scan(...any) error }) (*Item, error) {
	var it Item
	var fields string
	if err := row.Scan(&it.ID, &it.CategoryID, &it.Name, &it.Brand, &it.Status,
		&it.PurchaseDate, &it.PurchasePrice, &it.PartedDate, &it.PartedPrice,
		&it.Note, &fields, &it.CreatedAt, &it.UpdatedAt); err != nil {
		return nil, err
	}
	it.Fields = json.RawMessage(fields)
	return &it, nil
}

const itemCols = `id, category_id, name, brand, status, purchase_date, purchase_price, parted_date, parted_price, note, fields, created_at, updated_at`

func (s *Server) getItemByID(id int64) (*Item, error) {
	row := s.db.QueryRow(`SELECT `+itemCols+` FROM items WHERE id = ?`, id)
	it, err := scanItem(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return it, err
}

// setRelations replaces all relations of an item with the given ids,
// storing both directions so lookups only need item_id.
func (s *Server) setRelations(itemID int64, relatedIDs []int64) error {
	if _, err := s.db.Exec(`DELETE FROM item_relations WHERE item_id = ? OR related_item_id = ?`, itemID, itemID); err != nil {
		return err
	}
	seen := map[int64]bool{}
	for _, rid := range relatedIDs {
		if rid == itemID || seen[rid] {
			continue
		}
		seen[rid] = true
		var exists int
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM items WHERE id = ?`, rid).Scan(&exists); err != nil {
			return err
		}
		if exists == 0 {
			continue
		}
		if _, err := s.db.Exec(`INSERT OR IGNORE INTO item_relations (item_id, related_item_id) VALUES (?,?)`, itemID, rid); err != nil {
			return err
		}
		if _, err := s.db.Exec(`INSERT OR IGNORE INTO item_relations (item_id, related_item_id) VALUES (?,?)`, rid, itemID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) listItems(c *gin.Context) {
	where := []string{"1=1"}
	args := []any{}

	if v := c.Query("category_id"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "category_id 无效"})
			return
		}
		where = append(where, "category_id = ?")
		args = append(args, id)
	}
	if values := c.QueryArray("status"); len(values) > 0 {
		placeholders := make([]string, 0, len(values))
		for _, v := range values {
			if v != StatusCollecting && v != StatusParted {
				c.JSON(http.StatusBadRequest, gin.H{"error": "status 无效"})
				return
			}
			placeholders = append(placeholders, "?")
			args = append(args, v)
		}
		where = append(where, "status IN ("+strings.Join(placeholders, ",")+")")
	}
	if values := c.QueryArray("brand"); len(values) > 0 {
		placeholders := make([]string, 0, len(values))
		for _, v := range values {
			if v = strings.TrimSpace(v); v != "" {
				placeholders = append(placeholders, "?")
				args = append(args, v)
			}
		}
		if len(placeholders) > 0 {
			where = append(where, "brand IN ("+strings.Join(placeholders, ",")+")")
		}
	}
	if v := strings.TrimSpace(c.Query("q")); v != "" {
		where = append(where, "(name LIKE ? OR brand LIKE ?)")
		like := "%" + v + "%"
		args = append(args, like, like)
	}
	// 同一专属字段的多个值按 OR 匹配，不同字段之间按 AND 匹配。
	for key, values := range c.Request.URL.Query() {
		if !strings.HasPrefix(key, "f_") {
			continue
		}
		fk := strings.TrimPrefix(key, "f_")
		if !fieldKeyRe.MatchString(fk) {
			continue
		}
		placeholders := make([]string, 0, len(values))
		for _, v := range values {
			if v != "" {
				placeholders = append(placeholders, "?")
				args = append(args, v)
			}
		}
		if len(placeholders) > 0 {
			where = append(where, fmt.Sprintf("json_extract(fields, '$.%s') IN (%s)", fk, strings.Join(placeholders, ",")))
		}
	}

	// 排序：默认按购入时间倒序（无购入时间按创建时间），支持品牌/价格
	sort := c.DefaultQuery("sort", "purchase_date")
	var orderBy string
	switch sort {
	case "brand":
		orderBy = "brand ASC, id DESC"
	case "price_desc":
		orderBy = "purchase_price IS NULL, purchase_price DESC, id DESC"
	case "price_asc":
		orderBy = "purchase_price IS NULL, purchase_price ASC, id DESC"
	default: // purchase_date
		orderBy = "COALESCE(purchase_date, created_at) DESC, id DESC"
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "24"))
	if pageSize < 1 || pageSize > 200 {
		pageSize = 24
	}

	cond := strings.Join(where, " AND ")
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM items WHERE `+cond, args...).Scan(&total); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	query := `SELECT ` + itemCols + ` FROM items WHERE ` + cond + ` ORDER BY ` + orderBy + ` LIMIT ? OFFSET ?`
	qargs := append(append([]any{}, args...), pageSize, (page-1)*pageSize)
	rows, err := s.db.Query(query, qargs...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	items := []*Item{}
	for rows.Next() {
		it, err := scanItem(rows)
		if err != nil {
			rows.Close()
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		items = append(items, it)
	}
	rows.Close()

	cats := s.categoryMap()
	list := []gin.H{}
	for _, it := range items {
		entry := gin.H{
			"id":              it.ID,
			"category_id":     it.CategoryID,
			"name":            it.Name,
			"brand":           it.Brand,
			"status":          it.Status,
			"purchase_date":   it.PurchaseDate,
			"purchase_price":  it.PurchasePrice,
			"parted_date":     it.PartedDate,
			"parted_price":    it.PartedPrice,
			"note":            it.Note,
			"fields":          it.Fields,
			"created_at":      it.CreatedAt,
			"updated_at":      it.UpdatedAt,
			"cover_thumb_url": s.coverThumbURL(it.ID),
			"cover_url":       s.coverURL(it.ID),
		}
		if cat, ok := cats[it.CategoryID]; ok {
			entry["category"] = gin.H{
				"key": cat.Key, "name": cat.Name, "icon": cat.Icon, "color": cat.Color,
			}
		}
		list = append(list, entry)
	}
	c.JSON(http.StatusOK, gin.H{"total": total, "list": list})
}

func (s *Server) categoryMap() map[int64]*Category {
	m := map[int64]*Category{}
	rows, err := s.db.Query(`SELECT ` + categoryCols + ` FROM categories`)
	if err != nil {
		return m
	}
	defer rows.Close()
	for rows.Next() {
		cat, err := scanCategory(rows)
		if err == nil {
			m[cat.ID] = cat
		}
	}
	return m
}

// coverThumbURL returns the thumb URL of the item's first image (cover).
func (s *Server) coverThumbURL(itemID int64) string {
	var thumb string
	err := s.db.QueryRow(`SELECT thumb FROM images WHERE item_id = ? ORDER BY sort, id LIMIT 1`, itemID).Scan(&thumb)
	if err != nil || thumb == "" {
		return ""
	}
	return "/uploads/thumbs/" + thumb
}

// coverURL returns the original URL of the item's first image (cover).
func (s *Server) coverURL(itemID int64) string {
	var filename string
	err := s.db.QueryRow(`SELECT filename FROM images WHERE item_id = ? ORDER BY sort, id LIMIT 1`, itemID).Scan(&filename)
	if err != nil || filename == "" {
		return ""
	}
	return "/uploads/" + filename
}

func (s *Server) getItem(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	it, err := s.getItemByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if it == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "藏品不存在"})
		return
	}

	imgRows, err := s.db.Query(`SELECT id, item_id, filename, thumb, sort FROM images WHERE item_id = ? ORDER BY sort, id`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	images := []gin.H{}
	for imgRows.Next() {
		var img Image
		if err := imgRows.Scan(&img.ID, &img.ItemID, &img.Filename, &img.Thumb, &img.Sort); err != nil {
			imgRows.Close()
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		images = append(images, gin.H{
			"id":        img.ID,
			"url":       "/uploads/" + img.Filename,
			"thumb_url": "/uploads/thumbs/" + img.Thumb,
			"sort":      img.Sort,
		})
	}
	imgRows.Close()

	cats := s.categoryMap()
	relRows, err := s.db.Query(
		`SELECT i.id, i.name, i.category_id FROM item_relations r JOIN items i ON i.id = r.related_item_id WHERE r.item_id = ? ORDER BY r.id`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	type relRow struct {
		id    int64
		name  string
		catID int64
	}
	relList := []relRow{}
	for relRows.Next() {
		var r relRow
		if err := relRows.Scan(&r.id, &r.name, &r.catID); err != nil {
			relRows.Close()
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		relList = append(relList, r)
	}
	relRows.Close()

	relations := []gin.H{}
	for _, r := range relList {
		entry := gin.H{"id": r.id, "item_id": r.id, "name": r.name, "cover_thumb_url": s.coverThumbURL(r.id)}
		if cat, ok := cats[r.catID]; ok {
			entry["category_key"] = cat.Key
			entry["category_name"] = cat.Name
			entry["label"] = relationLabel(cats[it.CategoryID], cat.Key)
		}
		relations = append(relations, entry)
	}

	resp := gin.H{
		"id":              it.ID,
		"category_id":     it.CategoryID,
		"name":            it.Name,
		"brand":           it.Brand,
		"status":          it.Status,
		"purchase_date":   it.PurchaseDate,
		"purchase_price":  it.PurchasePrice,
		"parted_date":     it.PartedDate,
		"parted_price":    it.PartedPrice,
		"note":            it.Note,
		"fields":          it.Fields,
		"created_at":      it.CreatedAt,
		"updated_at":      it.UpdatedAt,
		"images":          images,
		"relations":       relations,
		"cover_url":       s.coverURL(id),
		"cover_thumb_url": s.coverThumbURL(id),
	}
	if cat, ok := cats[it.CategoryID]; ok {
		resp["category"] = gin.H{
			"key": cat.Key, "name": cat.Name, "icon": cat.Icon, "color": cat.Color,
		}
	}
	c.JSON(http.StatusOK, resp)
}

// relationLabel finds the label configured on the item's category for the target category key.
func relationLabel(cat *Category, targetKey string) string {
	if cat == nil {
		return ""
	}
	for _, r := range cat.Relations {
		if r.TargetKey == targetKey {
			return r.Label
		}
	}
	return ""
}

func (s *Server) createItem(c *gin.Context) {
	var in itemPayload
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误: " + err.Error()})
		return
	}
	msg, fields := in.validate()
	if msg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}
	if cat, _ := s.getCategoryByID(in.CategoryID); cat == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "品类不存在"})
		return
	}
	ts := now()
	res, err := s.db.Exec(
		`INSERT INTO items (category_id, name, brand, status, purchase_date, purchase_price, parted_date, parted_price, note, fields, created_at, updated_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		in.CategoryID, in.Name, in.Brand, in.Status, in.PurchaseDate, in.PurchasePrice,
		in.PartedDate, in.PartedPrice, in.Note, string(fields), ts, ts,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	id, _ := res.LastInsertId()
	if err := s.setRelations(id, in.RelatedIDs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	it, _ := s.getItemByID(id)
	c.JSON(http.StatusCreated, it)
}

func (s *Server) updateItem(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	existing, err := s.getItemByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "藏品不存在"})
		return
	}
	var in itemPayload
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求格式错误: " + err.Error()})
		return
	}
	msg, fields := in.validate()
	if msg != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": msg})
		return
	}
	if cat, _ := s.getCategoryByID(in.CategoryID); cat == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "品类不存在"})
		return
	}
	_, err = s.db.Exec(
		`UPDATE items SET category_id=?, name=?, brand=?, status=?, purchase_date=?, purchase_price=?,
		 parted_date=?, parted_price=?, note=?, fields=?, updated_at=? WHERE id=?`,
		in.CategoryID, in.Name, in.Brand, in.Status, in.PurchaseDate, in.PurchasePrice,
		in.PartedDate, in.PartedPrice, in.Note, string(fields), now(), id,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := s.setRelations(id, in.RelatedIDs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	it, _ := s.getItemByID(id)
	c.JSON(http.StatusOK, it)
}

func (s *Server) deleteItem(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的 id"})
		return
	}
	existing, err := s.getItemByID(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if existing == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "藏品不存在"})
		return
	}
	rows, err := s.db.Query(`SELECT filename, thumb FROM images WHERE item_id = ?`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	files := [][2]string{}
	for rows.Next() {
		var f, t string
		if err := rows.Scan(&f, &t); err == nil {
			files = append(files, [2]string{f, t})
		}
	}
	rows.Close()

	if _, err := s.db.Exec(`DELETE FROM item_relations WHERE item_id = ? OR related_item_id = ?`, id, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if _, err := s.db.Exec(`DELETE FROM items WHERE id = ?`, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	s.removeImageFiles(files)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
