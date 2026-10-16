package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"ydxt-go/internal/model"
	"ydxt-go/internal/utils"

	"github.com/gin-gonic/gin"
)

// Prevent teacher/admin primary keys from being used as users.id in learner APIs.
func AccountBusinessPolicy() gin.HandlerFunc {
	return func(c *gin.Context) {
		value, _ := c.Get("accountIdentity")
		identity, ok := value.(*utils.AccountIdentity)
		if !ok {
			c.AbortWithStatusJSON(401, gin.H{"code": 401, "msg": "账号身份缺失"})
			return
		}
		path := strings.TrimPrefix(c.FullPath(), "/api/v1")
		studentOnly := strings.HasPrefix(path, "/shop/") || strings.HasPrefix(path, "/course-progress/") ||
			path == "/lesson-progress" || path == "/course-orders" || path == "/courses/purchased" ||
			strings.HasSuffix(path, "/progress") || (strings.HasPrefix(path, "/homework/") && path != "/homework/media-play-url") ||
			((strings.HasPrefix(path, "/community/") || strings.HasPrefix(path, "/lesson-comments")) && c.Request.Method != "GET")
		if studentOnly && identity.AccountType != accountTypeUser {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"code": 403,
				"msg":  "该接口仅供学员账号使用，当前账号不能操作学员数据（" + c.Request.Method + " " + c.Request.URL.Path + "）",
			})
			return
		}
		c.Next()
	}
}

func AdminResourcePolicy() gin.HandlerFunc {
	return func(c *gin.Context) {
		if isTeacher, _ := currentTeacherScope(c); !isTeacher {
			c.Next()
			return
		}
		path := strings.TrimPrefix(c.FullPath(), "/api/v1/admin")
		method := c.Request.Method
		allowedList := method == "GET" && (path == "/dashboard/summary" || path == "/users" || path == "/orders" || path == "/teachers" || path == "/courses" || path == "/learning/tasks")
		if allowedList || path == "/courses" || strings.HasPrefix(path, "/courses/upload-") || strings.HasPrefix(path, "/homework/submissions") {
			c.Next()
			return
		}
		var courseID uint64
		var err error
		switch {
		case strings.HasPrefix(path, "/courses/:id/") || path == "/courses/:id":
			if id, ok := parseUintParam(c, "id"); ok {
				courseID = id
			} else {
				c.Abort()
				return
			}
		case strings.HasPrefix(path, "/cohorts/:id"):
			var cohort model.CourseCohort
			err = model.DB.Where("id = ?", c.Param("id")).First(&cohort).Error
			courseID = cohort.CourseID
		case strings.HasPrefix(path, "/chapters/:id"):
			var chapter model.CourseChapter
			err = model.DB.Where("id = ?", c.Param("id")).First(&chapter).Error
			courseID = chapter.CourseID
		case strings.HasPrefix(path, "/lessons/:id"):
			err = model.DB.Table("course_lessons l").Select("ch.course_id").Joins("JOIN course_chapters ch ON ch.id = l.chapter_id").Where("l.id = ?", c.Param("id")).Scan(&courseID).Error
		case strings.HasPrefix(path, "/learning/tasks/:id"):
			var task model.LearningTask
			err = model.DB.Where("id = ?", c.Param("id")).First(&task).Error
			courseID = task.CourseID
		case strings.HasPrefix(path, "/learning/questions/:id"):
			err = model.DB.Table("learning_task_questions q").Select("t.course_id").Joins("JOIN learning_tasks t ON t.id = q.task_id").Where("q.id = ?", c.Param("id")).Scan(&courseID).Error
		case path == "/learning/tasks" && method == "POST":
			raw, readErr := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20))
			c.Request.Body = io.NopCloser(bytes.NewReader(raw))
			var payload struct {
				CourseID uint64 `json:"course_id"`
			}
			if readErr != nil || json.Unmarshal(raw, &payload) != nil {
				c.AbortWithStatusJSON(400, gin.H{"code": 400, "msg": "任务参数格式无效"})
				return
			}
			courseID = payload.CourseID
		default:
			c.AbortWithStatusJSON(403, gin.H{"code": 403, "msg": "讲师账号没有该管理功能的权限"})
			return
		}
		if err != nil || courseID == 0 {
			c.AbortWithStatusJSON(404, gin.H{"code": 404, "msg": "教学资源不存在"})
			return
		}
		if _, ok := ensureCoursePermission(c, courseID); !ok {
			c.Abort()
			return
		}
		c.Next()
	}
}
