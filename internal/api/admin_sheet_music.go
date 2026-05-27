package api

import (
	"net/http"
	"strings"
	"ydxt-go/internal/model"

	"github.com/gin-gonic/gin"
)

type adminSheetMusicRequest struct {
	Title      string `json:"title" binding:"required"`
	Author     string `json:"author"`
	Instrument string `json:"instrument"`
	Difficulty int8   `json:"difficulty"`
	IsFree     int8   `json:"is_free"`
	CoverURL   string `json:"cover_url"`
	ContentURL string `json:"content_url"`
	Downloads  int    `json:"downloads"`
}

func normalizeDifficulty(difficulty int8) int8 {
	if difficulty < 1 {
		return 1
	}
	if difficulty > 5 {
		return 5
	}
	return difficulty
}

func normalizeSheetFree(isFree int8) int8 {
	if isFree == 0 {
		return 0
	}
	return 1
}

// AdminGetSheetMusics 获取后台曲谱列表
func AdminGetSheetMusics(c *gin.Context) {
	if !requireAdminAccount(c) {
		return
	}
	if model.DB == nil {
		c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": []sheetMusicResponse{}})
		return
	}

	var sheets []model.SheetMusic
	query := model.DB.Order("created_at desc")

	if keyword := strings.TrimSpace(c.Query("keyword")); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("title LIKE ? OR author LIKE ?", like, like)
	}
	if instrument := strings.TrimSpace(c.Query("instrument")); instrument != "" && instrument != "全部" {
		query = query.Where("instrument = ?", instrument)
	}
	if rawFree := strings.TrimSpace(c.Query("is_free")); rawFree != "" {
		if rawFree == "0" || rawFree == "1" {
			query = query.Where("is_free = ?", rawFree)
		}
	}

	if err := query.Find(&sheets).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "获取曲谱列表失败"})
		return
	}

	result := make([]sheetMusicResponse, 0, len(sheets))
	for _, item := range sheets {
		result = append(result, toSheetMusicResponse(item))
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": result})
}

// AdminGetSheetMusicDetail 获取后台曲谱详情
func AdminGetSheetMusicDetail(c *gin.Context) {
	if !requireAdminAccount(c) {
		return
	}

	id, ok := parseUint(c, "id")
	if !ok {
		return
	}

	var sheet model.SheetMusic
	if err := model.DB.Where("id = ?", id).First(&sheet).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "曲谱不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": toSheetMusicResponse(sheet)})
}

// AdminCreateSheetMusic 新增曲谱
func AdminCreateSheetMusic(c *gin.Context) {
	if !requireAdminAccount(c) {
		return
	}

	var req adminSheetMusicRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}

	sheet := model.SheetMusic{
		Title:      strings.TrimSpace(req.Title),
		Author:     strings.TrimSpace(req.Author),
		Instrument: strings.TrimSpace(req.Instrument),
		Difficulty: normalizeDifficulty(req.Difficulty),
		IsFree:     normalizeSheetFree(req.IsFree),
		CoverURL:   strings.TrimSpace(req.CoverURL),
		ContentURL: strings.TrimSpace(req.ContentURL),
		Downloads:  max(req.Downloads, 0),
	}

	if err := model.DB.Create(&sheet).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "新增曲谱失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": toSheetMusicResponse(sheet)})
}

// AdminUpdateSheetMusic 编辑曲谱
func AdminUpdateSheetMusic(c *gin.Context) {
	if !requireAdminAccount(c) {
		return
	}

	id, ok := parseUint(c, "id")
	if !ok {
		return
	}

	var req adminSheetMusicRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}

	updates := map[string]interface{}{
		"title":       strings.TrimSpace(req.Title),
		"author":      strings.TrimSpace(req.Author),
		"instrument":  strings.TrimSpace(req.Instrument),
		"difficulty":  normalizeDifficulty(req.Difficulty),
		"is_free":     normalizeSheetFree(req.IsFree),
		"cover_url":   strings.TrimSpace(req.CoverURL),
		"content_url": strings.TrimSpace(req.ContentURL),
		"downloads":   max(req.Downloads, 0),
	}

	result := model.DB.Model(&model.SheetMusic{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "更新曲谱失败"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "曲谱不存在"})
		return
	}

	var sheet model.SheetMusic
	_ = model.DB.Where("id = ?", id).First(&sheet).Error
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": toSheetMusicResponse(sheet)})
}

// AdminDeleteSheetMusic 删除曲谱
func AdminDeleteSheetMusic(c *gin.Context) {
	if !requireAdminAccount(c) {
		return
	}

	id, ok := parseUint(c, "id")
	if !ok {
		return
	}

	result := model.DB.Where("id = ?", id).Delete(&model.SheetMusic{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "删除曲谱失败"})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "曲谱不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success"})
}
