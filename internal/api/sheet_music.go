package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"ydxt-go/internal/model"

	"github.com/gin-gonic/gin"
)

type sheetMusicResponse struct {
	ID         uint64   `json:"id"`
	Title      string   `json:"title"`
	Author     string   `json:"author"`
	Instrument string   `json:"instrument"`
	Difficulty int8     `json:"difficulty"`
	IsFree     int8     `json:"is_free"`
	CoverURL   string   `json:"cover_url"`
	ContentURL string   `json:"content_url"`
	ContentURLs []string `json:"content_urls"`
	Downloads  int      `json:"downloads"`
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

func toSheetMusicResponse(item model.SheetMusic) sheetMusicResponse {
	return sheetMusicResponse{
		ID:          item.ID,
		Title:       item.Title,
		Author:      item.Author,
		Instrument:  item.Instrument,
		Difficulty:  item.Difficulty,
		IsFree:      item.IsFree,
		CoverURL:    item.CoverURL,
		ContentURL:  item.ContentURL,
		ContentURLs: parseSheetContentURLs(item.ContentURL),
		Downloads:   item.Downloads,
	}
}

// GetSheetMusics 获取曲谱列表
func GetSheetMusics(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": []sheetMusicResponse{}})
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

	var sheet model.SheetMusic
	if err := model.DB.Where("id = ?", id).First(&sheet).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "曲谱不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": toSheetMusicResponse(sheet)})
}
