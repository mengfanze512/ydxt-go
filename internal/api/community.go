package api

import (
	"net/http"
	"ydxt-go/internal/model"

	"github.com/gin-gonic/gin"
)

// GetPosts 获取社区帖子列表
func GetPosts(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "未连接数据库", "data": []model.CommunityPost{}})
		return
	}
	var posts []model.CommunityPost
	if err := model.DB.Where("status = ?", "normal").Order("created_at desc").Find(&posts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "获取帖子失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "success",
		"data": posts,
	})
}
