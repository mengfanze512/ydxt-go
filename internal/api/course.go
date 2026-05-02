package api

import (
	"net/http"
	"yuedi_edu/internal/model"

	"github.com/gin-gonic/gin"
)

// GetCourseList 获取公开课程列表 (支持前台分页和分类)
func GetCourseList(c *gin.Context) {
	category := c.Query("category") // 例如: "1", "2"
	difficulty := c.Query("difficulty")       // 例如: "1", "2"

	var courses []model.Course
	query := model.DB.Where("status = ?", 1).Where("is_deleted = ?", 0)

	if category != "" && category != "0" && category != "全部" {
		query = query.Where("category = ?", category)
	}
	if difficulty != "" && difficulty != "0" {
		query = query.Where("difficulty = ?", difficulty)
	}

	if err := query.Order("created_at desc").Find(&courses).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "获取课程列表失败"})
		return
	}

	// 为前台组装讲师名字(简单处理，实际应联表查询)
	for i, course := range courses {
		var teacher model.User
		if err := model.DB.Where("id = ?", course.TeacherID).First(&teacher).Error; err == nil {
			courses[i].TeacherName = teacher.Nickname
		} else {
			courses[i].TeacherName = "特邀讲师"
		}
	}

	// 如果没有数据，返回一些 mock 数据用于前端演示
	if len(courses) == 0 {
		courses = []model.Course{
			{ID: 1, Title: "零基础吉他入门 30 天精通", Cover: "https://img.yzcdn.cn/vant/cat.jpeg", Price: 19900, TeacherName: "张三名师", Category: 1, Difficulty: 1, Type: 1, StudentCount: 1280},
			{ID: 2, Title: "钢琴考级冲刺班 (七级-十级)", Cover: "https://img.yzcdn.cn/vant/cat.jpeg", Price: 29900, TeacherName: "李四名师", Category: 2, Difficulty: 3, Type: 1, StudentCount: 860},
			{ID: 3, Title: "架子鼓名曲精讲", Cover: "https://img.yzcdn.cn/vant/cat.jpeg", Price: 9900, TeacherName: "王五名师", Category: 3, Difficulty: 2, Type: 1, StudentCount: 320},
			{ID: 4, Title: "钢琴 1对1 在线私教陪练", Cover: "https://img.yzcdn.cn/vant/cat.jpeg", Price: 30000, TeacherName: "特邀讲师", Category: 2, Difficulty: 2, Type: 2, StudentCount: 50},
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": courses,
	})
}
