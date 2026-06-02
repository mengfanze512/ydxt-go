package api

import (
	"log"
	"net/http"
	"strconv"
	"strings"
	"ydxt-go/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type lessonCommentResponse struct {
	ID              uint64                  `json:"id"`
	CourseID        uint64                  `json:"course_id"`
	LessonID        uint64                  `json:"lesson_id"`
	UserID          uint64                  `json:"user_id"`
	ParentID        uint64                  `json:"parent_id"`
	ReplyToUserID   uint64                  `json:"reply_to_user_id"`
	AuthorNickname  string                  `json:"author_nickname"`
	AuthorAvatar    string                  `json:"author_avatar"`
	ReplyToNickname string                  `json:"reply_to_nickname"`
	Content         string                  `json:"content"`
	LikesCount      int                     `json:"likes_count"`
	IsLiked         bool                    `json:"is_liked"`
	Status          string                  `json:"status"`
	CreatedAt       string                  `json:"created_at"`
	Replies         []lessonCommentResponse `json:"replies,omitempty"`
}

type createLessonCommentRequest struct {
	CourseID         uint64 `json:"course_id"`
	LessonID         uint64 `json:"lesson_id"`
	Content          string `json:"content"`
	ReplyToCommentID uint64 `json:"reply_to_comment_id"`
}

func ensureCourseLessonExists(courseID uint64, lessonID uint64) bool {
	if model.DB == nil || courseID == 0 || lessonID == 0 {
		return false
	}
	type lessonRow struct {
		ID uint64 `gorm:"column:id"`
	}
	var row lessonRow
	err := model.DB.Table("course_lessons cl").
		Select("cl.id").
		Joins("JOIN course_chapters cc ON cc.id = cl.chapter_id").
		Where("cl.id = ? AND cc.course_id = ?", lessonID, courseID).
		Take(&row).Error
	return err == nil && row.ID > 0
}

func extractLessonCommentUserIDs(comments []model.CourseLessonComment) []uint64 {
	m := map[uint64]struct{}{}
	for _, item := range comments {
		if item.UserID > 0 {
			m[item.UserID] = struct{}{}
		}
		if item.ReplyToUserID > 0 {
			m[item.ReplyToUserID] = struct{}{}
		}
	}
	ids := make([]uint64, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	return ids
}

func extractLessonCommentIDs(comments []model.CourseLessonComment) []uint64 {
	ids := make([]uint64, 0, len(comments))
	for _, item := range comments {
		ids = append(ids, item.ID)
	}
	return ids
}

func buildLessonCommentLikedMap(userID uint64, commentIDs []uint64) map[uint64]bool {
	out := map[uint64]bool{}
	if model.DB == nil || userID == 0 || len(commentIDs) == 0 {
		return out
	}
	var rows []model.CourseLessonCommentLike
	if err := model.DB.Where("user_id = ? AND comment_id IN ?", userID, commentIDs).Find(&rows).Error; err != nil {
		return out
	}
	for _, row := range rows {
		out[row.CommentID] = true
	}
	return out
}

func toLessonCommentResponse(item model.CourseLessonComment, nicknameMap map[uint64]string, avatarMap map[uint64]string, likedMap map[uint64]bool) lessonCommentResponse {
	return lessonCommentResponse{
		ID:              item.ID,
		CourseID:        item.CourseID,
		LessonID:        item.LessonID,
		UserID:          item.UserID,
		ParentID:        item.ParentID,
		ReplyToUserID:   item.ReplyToUserID,
		AuthorNickname:  nicknameMap[item.UserID],
		AuthorAvatar:    avatarMap[item.UserID],
		ReplyToNickname: nicknameMap[item.ReplyToUserID],
		Content:         item.Content,
		LikesCount:      item.LikesCount,
		IsLiked:         likedMap[item.ID],
		Status:          item.Status,
		CreatedAt:       item.CreatedAt.Format("2006-01-02 15:04:05"),
	}
}

// GetLessonComments 获取课程小节评论列表（二级）
func GetLessonComments(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "未连接数据库", "data": []lessonCommentResponse{}})
		return
	}

	courseID, _ := strconv.ParseUint(c.Query("course_id"), 10, 64)
	lessonID, _ := strconv.ParseUint(c.Query("lesson_id"), 10, 64)
	if courseID == 0 || lessonID == 0 || !ensureCourseLessonExists(courseID, lessonID) {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "课时不存在"})
		return
	}

	userID, _ := currentUserID(c)
	var comments []model.CourseLessonComment
	if err := model.DB.Where("course_id = ? AND lesson_id = ? AND status <> ?", courseID, lessonID, model.CourseLessonCommentStatusDeleted).
		Order("created_at asc").
		Find(&comments).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "获取评论失败"})
		return
	}

	nicknameMap, avatarMap := buildNicknameAvatarMap(extractLessonCommentUserIDs(comments))
	likedMap := buildLessonCommentLikedMap(userID, extractLessonCommentIDs(comments))
	topMap := map[uint64]*lessonCommentResponse{}
	topList := make([]lessonCommentResponse, 0)
	replies := make([]model.CourseLessonComment, 0)

	for _, item := range comments {
		if item.ParentID == 0 {
			resp := toLessonCommentResponse(item, nicknameMap, avatarMap, likedMap)
			topList = append(topList, resp)
			topMap[item.ID] = &topList[len(topList)-1]
			continue
		}
		replies = append(replies, item)
	}
	for _, item := range replies {
		parent := topMap[item.ParentID]
		if parent == nil {
			continue
		}
		parent.Replies = append(parent.Replies, toLessonCommentResponse(item, nicknameMap, avatarMap, likedMap))
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": topList})
}

// CreateLessonComment 发布课程小节评论/回复
func CreateLessonComment(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "未连接数据库", "data": nil})
		return
	}
	userID, ok := currentUserID(c)
	if !ok || userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "未登录"})
		return
	}

	var req createLessonCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	if req.CourseID == 0 || req.LessonID == 0 || !ensureCourseLessonExists(req.CourseID, req.LessonID) {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "课时不存在"})
		return
	}

	req.Content = strings.TrimSpace(req.Content)
	if req.Content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "评论不能为空"})
		return
	}
	if hitSensitiveWord(req.Content) {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "包含敏感词"})
		return
	}

	var parentID uint64
	var replyToUserID uint64
	if req.ReplyToCommentID > 0 {
		var replyTo model.CourseLessonComment
		if err := model.DB.First(&replyTo, req.ReplyToCommentID).Error; err != nil ||
			replyTo.Status == model.CourseLessonCommentStatusDeleted ||
			replyTo.CourseID != req.CourseID ||
			replyTo.LessonID != req.LessonID {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "回复对象不存在"})
			return
		}
		replyToUserID = replyTo.UserID
		if replyTo.ParentID == 0 {
			parentID = replyTo.ID
		} else {
			parentID = replyTo.ParentID
		}

		var root model.CourseLessonComment
		if err := model.DB.Select("id, parent_id, status").First(&root, parentID).Error; err != nil ||
			root.ParentID != 0 ||
			root.Status == model.CourseLessonCommentStatusDeleted {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "回复对象不存在"})
			return
		}
	}

	comment := model.CourseLessonComment{
		CourseID:      req.CourseID,
		LessonID:      req.LessonID,
		UserID:        userID,
		ParentID:      parentID,
		ReplyToUserID: replyToUserID,
		Content:       req.Content,
		Status:        model.CourseLessonCommentStatusApproved,
	}
	if err := model.DB.Create(&comment).Error; err != nil {
		log.Printf("CreateLessonComment failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "发布失败"})
		return
	}

	nicknameMap, avatarMap := buildNicknameAvatarMap([]uint64{userID, replyToUserID})
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": toLessonCommentResponse(comment, nicknameMap, avatarMap, map[uint64]bool{comment.ID: false})})
}

// LikeLessonComment 点赞课程小节评论
func LikeLessonComment(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "未连接数据库", "data": nil})
		return
	}
	userID, ok := currentUserID(c)
	if !ok || userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "未登录"})
		return
	}

	commentID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if commentID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}

	var comment model.CourseLessonComment
	if err := model.DB.First(&comment, commentID).Error; err != nil || comment.Status == model.CourseLessonCommentStatusDeleted {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "评论不存在"})
		return
	}

	err := model.DB.Transaction(func(tx *gorm.DB) error {
		var exists model.CourseLessonCommentLike
		if err := tx.Where("comment_id = ? AND user_id = ?", commentID, userID).Take(&exists).Error; err == nil {
			return nil
		} else if err != nil && err != gorm.ErrRecordNotFound {
			return err
		}
		if err := tx.Create(&model.CourseLessonCommentLike{CommentID: commentID, UserID: userID}).Error; err != nil {
			return err
		}
		return tx.Model(&model.CourseLessonComment{}).Where("id = ?", commentID).UpdateColumn("likes_count", gorm.Expr("likes_count + 1")).Error
	})
	if err != nil {
		log.Printf("LikeLessonComment failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "点赞失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": true})
}

// UnlikeLessonComment 取消点赞课程小节评论
func UnlikeLessonComment(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "未连接数据库", "data": nil})
		return
	}
	userID, ok := currentUserID(c)
	if !ok || userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "未登录"})
		return
	}

	commentID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if commentID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}

	var comment model.CourseLessonComment
	if err := model.DB.First(&comment, commentID).Error; err != nil || comment.Status == model.CourseLessonCommentStatusDeleted {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "评论不存在"})
		return
	}

	err := model.DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Where("comment_id = ? AND user_id = ?", commentID, userID).Delete(&model.CourseLessonCommentLike{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		return tx.Model(&model.CourseLessonComment{}).Where("id = ?", commentID).
			UpdateColumn("likes_count", gorm.Expr("CASE WHEN likes_count > 0 THEN likes_count - 1 ELSE 0 END")).Error
	})
	if err != nil {
		log.Printf("UnlikeLessonComment failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "取消点赞失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": true})
}
