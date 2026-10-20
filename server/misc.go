package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

func (s *Server) getFilters(c *gin.Context) {
	catID, err := strconv.ParseInt(c.Query("category_id"), 10, 64)
	if err != nil || catID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "category_id 无效"})
		return
	}
	cat, err := s.getCategoryByID(catID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if cat == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "品类不存在"})
		return
	}

	brandRows, err := s.db.Query(
		`SELECT DISTINCT brand FROM items WHERE category_id = ? AND brand != '' ORDER BY brand`, catID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	brands := []string{}
	for brandRows.Next() {
		var b string
		if err := brandRows.Scan(&b); err == nil {
			brands = append(brands, b)
		}
	}
	brandRows.Close()

	fields := map[string][]string{}
	for _, fd := range cat.Fields {
		if !fieldKeyRe.MatchString(fd.Key) {
			continue
		}
		rows, err := s.db.Query(fmt.Sprintf(
			`SELECT DISTINCT json_extract(fields, '$.%s') AS v FROM items
			 WHERE category_id = ? AND json_type(fields, '$.%s') IN ('text','integer','real') ORDER BY v`,
			fd.Key, fd.Key), catID)
		if err != nil {
			continue
		}
		vals := []string{}
		for rows.Next() {
			var v any
			if err := rows.Scan(&v); err == nil && v != nil {
				vals = append(vals, fmt.Sprintf("%v", v))
			}
		}
		rows.Close()
		fields[fd.Key] = vals
	}
	c.JSON(http.StatusOK, gin.H{"brands": brands, "fields": fields})
}

func (s *Server) getStats(c *gin.Context) {
	var totalCollecting int
	var totalValue float64
	if err := s.db.QueryRow(
		`SELECT COUNT(*), COALESCE(SUM(purchase_price), 0) FROM items WHERE status = 'collecting'`,
	).Scan(&totalCollecting, &totalValue); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	rows, err := s.db.Query(`SELECT ` + categoryCols + ` FROM categories ORDER BY sort, id`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	catList := []*Category{}
	for rows.Next() {
		cat, err := scanCategory(rows)
		if err != nil {
			rows.Close()
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		catList = append(catList, cat)
	}
	rows.Close()

	type agg struct {
		count int
		value float64
	}
	aggs := map[int64]agg{}
	aggRows, err := s.db.Query(
		`SELECT category_id, COUNT(*), COALESCE(SUM(purchase_price), 0) FROM items WHERE status = 'collecting' GROUP BY category_id`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	for aggRows.Next() {
		var catID int64
		var a agg
		if err := aggRows.Scan(&catID, &a.count, &a.value); err == nil {
			aggs[catID] = a
		}
	}
	aggRows.Close()

	cats := []gin.H{}
	for _, cat := range catList {
		a := aggs[cat.ID]
		cats = append(cats, gin.H{
			"id": cat.ID, "key": cat.Key, "name": cat.Name, "icon": cat.Icon,
			"color": cat.Color, "count": a.count, "total_value": a.value,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"total_collecting": totalCollecting,
		"total_value":      totalValue,
		"categories":       cats,
	})
}

// spendingStats 按品类汇总花费与结缘情况（仅管理端可见）。
func (s *Server) spendingStats(c *gin.Context) {
	rows, err := s.db.Query(`SELECT ` + categoryCols + ` FROM categories ORDER BY sort, id`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	catList := []*Category{}
	for rows.Next() {
		cat, err := scanCategory(rows)
		if err != nil {
			rows.Close()
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		catList = append(catList, cat)
	}
	rows.Close()

	type agg struct {
		totalCount    int
		spentTotal    float64 // 全部购入价合计
		collectingCnt int
		spentKeeping  float64 // 仍在收藏的购入价合计
		partedCnt     int
		spentParted   float64 // 已结缘藏品的原购入价合计
		partedAmount  float64 // 结缘收入合计
	}
	aggs := map[int64]*agg{}
	aggRows, err := s.db.Query(`
		SELECT category_id,
			COUNT(*),
			COALESCE(SUM(purchase_price), 0),
			COALESCE(SUM(CASE WHEN status = 'collecting' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'collecting' THEN purchase_price ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'parted' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'parted' THEN purchase_price ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status = 'parted' THEN COALESCE(parted_price, 0) ELSE 0 END), 0)
		FROM items GROUP BY category_id`)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	for aggRows.Next() {
		var catID int64
		a := &agg{}
		if err := aggRows.Scan(&catID, &a.totalCount, &a.spentTotal, &a.collectingCnt,
			&a.spentKeeping, &a.partedCnt, &a.spentParted, &a.partedAmount); err == nil {
			aggs[catID] = a
		}
	}
	aggRows.Close()

	var summary agg
	list := []gin.H{}
	for _, cat := range catList {
		a := aggs[cat.ID]
		if a == nil {
			a = &agg{}
		}
		summary.totalCount += a.totalCount
		summary.spentTotal += a.spentTotal
		summary.collectingCnt += a.collectingCnt
		summary.spentKeeping += a.spentKeeping
		summary.partedCnt += a.partedCnt
		summary.spentParted += a.spentParted
		summary.partedAmount += a.partedAmount
		list = append(list, gin.H{
			"id": cat.ID, "key": cat.Key, "name": cat.Name,
			"icon": cat.Icon, "color": cat.Color,
			"total_count":      a.totalCount,
			"spent_total":      a.spentTotal,
			"collecting_count": a.collectingCnt,
			"spent_collecting": a.spentKeeping,
			"parted_count":     a.partedCnt,
			"spent_parted":     a.spentParted,
			"parted_amount":    a.partedAmount,
			"parted_net":       a.partedAmount - a.spentParted,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"categories": list,
		"summary": gin.H{
			"total_count":      summary.totalCount,
			"spent_total":      summary.spentTotal,
			"collecting_count": summary.collectingCnt,
			"spent_collecting": summary.spentKeeping,
			"parted_count":     summary.partedCnt,
			"spent_parted":     summary.spentParted,
			"parted_amount":    summary.partedAmount,
			"parted_net":       summary.partedAmount - summary.spentParted,
		},
	})
}

func (s *Server) backup(c *gin.Context) {
	tmp := filepath.Join(os.TempDir(), fmt.Sprintf("inkcollection-backup-%d.db", time.Now().UnixNano()))
	if _, err := s.db.Exec(`VACUUM INTO ?`, tmp); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "备份失败: " + err.Error()})
		return
	}
	defer os.Remove(tmp)
	name := "collection-backup-" + time.Now().Format("20060102") + ".db"
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
	c.File(tmp)
}
