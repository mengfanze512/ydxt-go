package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"ydxt-go/internal/model"

	"github.com/gin-gonic/gin"
)

var (
	sheetMusicColumnsOnce sync.Once
	sheetMusicColumns     map[string]bool
)

type sheetMusicResponse struct {
	ID          uint64   `json:"id"`
	Title       string   `json:"title"`
	Author      string   `json:"author"`
	Instrument  string   `json:"instrument"`
	Difficulty  int8     `json:"difficulty"`
	IsFree      int8     `json:"is_free"`
	Price       float64  `json:"price"`
	CoverURL    string   `json:"cover_url"`
	ContentURL  string   `json:"content_url"`
	ContentURLs []string `json:"content_urls"`
	Downloads   int      `json:"downloads"`
}

type legacySheetMusicRecord struct {
	ID         uint64 `gorm:"column:id"`
	Title      string `gorm:"column:title"`
	Instrument string `gorm:"column:instrument"`
	Difficulty int8   `gorm:"column:difficulty"`
	Price      int    `gorm:"column:price"`
	ImgURL     string `gorm:"column:img_url"`
	XMLURL     string `gorm:"column:xml_url"`
	Status     int8   `gorm:"column:status"`
	IsDeleted  int8   `gorm:"column:is_deleted"`
}

func loadSheetMusicColumns() {
	sheetMusicColumns = map[string]bool{}
	if model.DB == nil {
		return
	}
	var columns []struct {
		Field string `gorm:"column:Field"`
	}
	if err := model.DB.Raw("SHOW COLUMNS FROM sheet_musics").Scan(&columns).Error; err != nil {
		return
	}
	for _, column := range columns {
		sheetMusicColumns[strings.TrimSpace(column.Field)] = true
	}
}

func hasSheetMusicColumn(column string) bool {
	sheetMusicColumnsOnce.Do(loadSheetMusicColumns)
	return sheetMusicColumns[column]
}

func isLegacySheetMusicSchema() bool {
	return hasSheetMusicColumn("img_url") && !hasSheetMusicColumn("cover_url")
}

func legacySheetMusicSelectFields() string {
	fields := []string{"id", "title", "instrument", "difficulty", "img_url", "xml_url", "status", "is_deleted"}
	if hasSheetMusicColumn("price") {
		fields = append(fields, "price")
	}
	return strings.Join(fields, ", ")
}

func firstSheetMusicNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func parseSheetContentURLs(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{}
	}
	var urls []string
	if strings.HasPrefix(raw, "[") {
		if err := json.Unmarshal([]byte(raw), &urls); err == nil {
			return urls
		}
	}
	return []string{raw}
}

func firstSheetContentURL(raw string) string {
	urls := parseSheetContentURLs(raw)
	if len(urls) == 0 {
		return ""
	}
	return strings.TrimSpace(urls[0])
}

func toSheetMusicResponse(item model.SheetMusic) sheetMusicResponse {
	return sheetMusicResponse{
		ID:          item.ID,
		Title:       item.Title,
		Author:      item.Author,
		Instrument:  item.Instrument,
		Difficulty:  item.Difficulty,
		IsFree:      item.IsFree,
		Price:       formatGoodsPrice(item.Price),
		CoverURL:    item.CoverURL,
		ContentURL:  item.ContentURL,
		ContentURLs: parseSheetContentURLs(item.ContentURL),
		Downloads:   item.Downloads,
	}
}

func toLegacySheetMusicResponse(item legacySheetMusicRecord) sheetMusicResponse {
	contentURL := strings.TrimSpace(item.XMLURL)
	coverURL := firstSheetMusicNonEmpty(item.ImgURL, contentURL)
	isFree := int8(1)
	if item.Price > 0 {
		isFree = 0
	}
	return sheetMusicResponse{
		ID:          item.ID,
		Title:       item.Title,
		Author:      "",
		Instrument:  item.Instrument,
		Difficulty:  item.Difficulty,
		IsFree:      isFree,
		Price:       formatGoodsPrice(item.Price),
		CoverURL:    coverURL,
		ContentURL:  contentURL,
		ContentURLs: parseSheetContentURLs(contentURL),
		Downloads:   0,
	}
}

// GetSheetMusics 获取曲谱列表
func GetSheetMusics(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": []sheetMusicResponse{}})
		return
	}

	if isLegacySheetMusicSchema() {
		var sheets []legacySheetMusicRecord
		query := model.DB.Table("sheet_musics").
			Select(legacySheetMusicSelectFields()).
			Where("is_deleted = ?", 0).
			Where("status = ?", 1).
			Order("created_at desc")
		if instrument := strings.TrimSpace(c.Query("instrument")); instrument != "" && instrument != "全部" {
			query = query.Where("instrument = ?", instrument)
		}
		if err := query.Scan(&sheets).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "获取曲谱失败"})
			return
		}
		result := make([]sheetMusicResponse, 0, len(sheets))
		for _, item := range sheets {
			result = append(result, toLegacySheetMusicResponse(item))
		}
		c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": result})
		return
	}

	var sheets []model.SheetMusic
	query := model.DB.Order("created_at desc")
	if instrument := strings.TrimSpace(c.Query("instrument")); instrument != "" && instrument != "全部" {
		query = query.Where("instrument = ?", instrument)
	}
	if err := query.Find(&sheets).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "获取曲谱失败"})
		return
	}

	result := make([]sheetMusicResponse, 0, len(sheets))
	for _, item := range sheets {
		result = append(result, toSheetMusicResponse(item))
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": result})
}

// GetSheetMusicDetail 获取曲谱详情
func GetSheetMusicDetail(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": nil})
		return
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "曲谱ID无效"})
		return
	}

	if isLegacySheetMusicSchema() {
		var sheet legacySheetMusicRecord
		if err := model.DB.Table("sheet_musics").
			Select(legacySheetMusicSelectFields()).
			Where("id = ? AND is_deleted = ? AND status = ?", id, 0, 1).
			First(&sheet).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "曲谱不存在"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": toLegacySheetMusicResponse(sheet)})
		return
	}

	var sheet model.SheetMusic
	if err := model.DB.Where("id = ?", id).First(&sheet).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "曲谱不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": toSheetMusicResponse(sheet)})
}
