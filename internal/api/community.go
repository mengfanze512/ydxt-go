package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"ydxt-go/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type communityPostResponse struct {
	ID             uint64   `json:"id"`
	UserID         uint64   `json:"user_id"`
	AuthorNickname string   `json:"author_nickname"`
	AuthorAvatar   string   `json:"author_avatar"`
	Title          string   `json:"title"`
	Content        string   `json:"content"`
	Images         []string `json:"images"`
	VideoURL       string   `json:"video_url"`
	VideoCoverURL  string   `json:"video_cover_url"`
	LikesCount     int      `json:"likes_count"`
	FavoritesCount int      `json:"favorites_count"`
	CommentsCount  int      `json:"comments_count"`
	IsPinned       int8     `json:"is_pinned"`
	IsFeatured     int8     `json:"is_featured"`
	IsLiked        bool     `json:"is_liked"`
	IsFavorited    bool     `json:"is_favorited"`
	Status         string   `json:"status"`
	RejectReason   string   `json:"reject_reason"`
	CreatedAt      string   `json:"created_at"`
}

type communityListResponse struct {
	List     []communityPostResponse `json:"list"`
	Total    int64                   `json:"total"`
	Page     int                     `json:"page"`
	PageSize int                     `json:"page_size"`
}

func parseJSONStringArray(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{}
	}
	if strings.HasPrefix(raw, "[") {
		var out []string
		if err := json.Unmarshal([]byte(raw), &out); err == nil {
			return out
		}
	}
	return []string{raw}
}

func toCommunityPostResponse(item model.CommunityPost, nickname string, avatar string) communityPostResponse {
	return communityPostResponse{
		ID:             item.ID,
		UserID:         item.UserID,
		AuthorNickname: nickname,
		AuthorAvatar:   avatar,
		Title:          item.Title,
		Content:        item.Content,
		Images:         parseJSONStringArray(item.Images),
		VideoURL:       item.VideoURL,
		VideoCoverURL:  item.VideoCoverURL,
		LikesCount:     item.LikesCount,
		FavoritesCount: item.FavoritesCount,
		CommentsCount:  item.CommentsCount,
		IsPinned:       item.IsPinned,
		IsFeatured:     item.IsFeatured,
		Status:         item.Status,
		RejectReason:   item.RejectReason,
		CreatedAt:      item.CreatedAt.Format("2006-01-02 15:04:05"),
	}
}

type communityPostUpsertRequest struct {
	Title         string   `json:"title"`
	Content       string   `json:"content"`
	Images        []string `json:"images"`
	VideoURL      string   `json:"video_url"`
	VideoCoverURL string   `json:"video_cover_url"`
}

func currentUserID(c *gin.Context) (uint64, bool) {
	userIDAny, ok := c.Get("userID")
	if !ok {
		return 0, false
	}
	userID, ok := userIDAny.(uint64)
	if ok {
		return userID, true
	}
	if v, ok := userIDAny.(int64); ok && v > 0 {
		return uint64(v), true
	}
	return 0, false
}

func isAdminAccount(c *gin.Context) bool {
	accountType, _ := c.Get("accountType")
	return accountType == accountTypeAdmin
}

func getPagination(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("page_size"))
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 20
	}
	return page, pageSize
}

func buildNicknameAvatarMap(userIDs []uint64) (map[uint64]string, map[uint64]string) {
	nicknameMap := map[uint64]string{}
	avatarMap := map[uint64]string{}
	if len(userIDs) == 0 || model.DB == nil {
		return nicknameMap, avatarMap
	}
	type userLite struct {
		ID       uint64 `gorm:"column:id"`
		Nickname string `gorm:"column:nickname"`
		Avatar   string `gorm:"column:avatar"`
	}
	var rows []userLite
	if err := model.DB.Table("users").Select("id, nickname, avatar").Where("id IN ?", userIDs).Scan(&rows).Error; err != nil {
		return nicknameMap, avatarMap
	}
	for _, row := range rows {
		nicknameMap[row.ID] = row.Nickname
		avatarMap[row.ID] = row.Avatar
	}
	return nicknameMap, avatarMap
}

func extractIDs(posts []model.CommunityPost) []uint64 {
	ids := make([]uint64, 0, len(posts))
	for _, item := range posts {
		ids = append(ids, item.ID)
	}
	return ids
}

func extractUserIDs(posts []model.CommunityPost) []uint64 {
	m := map[uint64]struct{}{}
	for _, item := range posts {
		m[item.UserID] = struct{}{}
	}
	ids := make([]uint64, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	return ids
}

func buildLikedMap(userID uint64, postIDs []uint64) map[uint64]bool {
	out := map[uint64]bool{}
	if model.DB == nil || userID == 0 || len(postIDs) == 0 {
		return out
	}
	var rows []model.CommunityPostLike
	if err := model.DB.Where("user_id = ? AND post_id IN ?", userID, postIDs).Find(&rows).Error; err != nil {
		return out
	}
	for _, row := range rows {
		out[row.PostID] = true
	}
	return out
}

func buildFavoritedMap(userID uint64, postIDs []uint64) map[uint64]bool {
	out := map[uint64]bool{}
	if model.DB == nil || userID == 0 || len(postIDs) == 0 {
		return out
	}
	var rows []model.CommunityPostFavorite
	if err := model.DB.Where("user_id = ? AND post_id IN ?", userID, postIDs).Find(&rows).Error; err != nil {
		return out
	}
	for _, row := range rows {
		out[row.PostID] = true
	}
	return out
}

// GetCommunityPosts 教学端：获取帖子列表（默认：已审核通过 + 自己发布的全部）
func GetCommunityPosts(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "未连接数据库", "data": communityListResponse{List: []communityPostResponse{}, Total: 0, Page: 1, PageSize: 20}})
		return
	}

	userID, _ := currentUserID(c)
	onlyMine := c.Query("mine") == "1" || strings.ToLower(c.Query("mine")) == "true"
	page, pageSize := getPagination(c)

	query := model.DB.Model(&model.CommunityPost{})
	if onlyMine && userID > 0 {
		query = query.Where("user_id = ? AND status <> ?", userID, model.CommunityPostStatusDeleted)
	} else if userID > 0 {
		query = query.Where("(status = ? OR user_id = ?) AND status <> ?", model.CommunityPostStatusApproved, userID, model.CommunityPostStatusDeleted)
	} else {
		query = query.Where("status = ?", model.CommunityPostStatusApproved)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "获取帖子失败"})
		return
	}

	var posts []model.CommunityPost
	if err := query.Order("is_pinned desc, created_at desc").Limit(pageSize).Offset((page - 1) * pageSize).Find(&posts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "获取帖子失败"})
		return
	}

	postIDs := extractIDs(posts)
	likedMap := buildLikedMap(userID, postIDs)
	favoritedMap := buildFavoritedMap(userID, postIDs)
	nicknameMap, avatarMap := buildNicknameAvatarMap(extractUserIDs(posts))

	resp := make([]communityPostResponse, 0, len(posts))
	for _, item := range posts {
		row := toCommunityPostResponse(item, nicknameMap[item.UserID], avatarMap[item.UserID])
		row.IsLiked = likedMap[item.ID]
		row.IsFavorited = favoritedMap[item.ID]
		resp = append(resp, row)
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
		"data": communityListResponse{List: resp, Total: total, Page: page, PageSize: pageSize},
	})
}

// GetCommunityPostDetail 教学端：获取帖子详情（允许作者查看自己未审核帖子）
func GetCommunityPostDetail(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "未连接数据库", "data": nil})
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}

	var post model.CommunityPost
	if err := model.DB.First(&post, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "帖子不存在"})
		return
	}

	userID, _ := currentUserID(c)
	if post.Status != model.CommunityPostStatusApproved && post.UserID != userID && !isAdminAccount(c) {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "帖子不存在"})
		return
	}
	if post.Status == model.CommunityPostStatusDeleted {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "帖子不存在"})
		return
	}

	nicknameMap, avatarMap := buildNicknameAvatarMap([]uint64{post.UserID})
	resp := toCommunityPostResponse(post, nicknameMap[post.UserID], avatarMap[post.UserID])
	resp.IsLiked = buildLikedMap(userID, []uint64{post.ID})[post.ID]
	resp.IsFavorited = buildFavoritedMap(userID, []uint64{post.ID})[post.ID]
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": resp})
}

// CreateCommunityPost 教学端：发布帖子（进入待审核）
func CreateCommunityPost(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "未连接数据库", "data": nil})
		return
	}
	userID, ok := currentUserID(c)
	if !ok || userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "未登录"})
		return
	}

	var req communityPostUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	req.Content = strings.TrimSpace(req.Content)
	if req.Title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "标题不能为空"})
		return
	}
	if req.Content == "" && len(req.Images) == 0 && strings.TrimSpace(req.VideoURL) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "内容不能为空"})
		return
	}

	images := req.Images
	if images == nil {
		images = []string{}
	}
	imagesJSON, _ := json.Marshal(images)
	post := model.CommunityPost{
		UserID:        userID,
		Title:         req.Title,
		Content:       req.Content,
		Images:        string(imagesJSON),
		VideoURL:      strings.TrimSpace(req.VideoURL),
		VideoCoverURL: strings.TrimSpace(req.VideoCoverURL),
		Status:        model.CommunityPostStatusPending,
	}
	if err := model.DB.Create(&post).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "发布失败"})
		return
	}

	nicknameMap, avatarMap := buildNicknameAvatarMap([]uint64{userID})
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": toCommunityPostResponse(post, nicknameMap[userID], avatarMap[userID])})
}

// UpdateCommunityPost 教学端：编辑帖子（编辑后回到待审核）
func UpdateCommunityPost(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "未连接数据库", "data": nil})
		return
	}
	userID, ok := currentUserID(c)
	if !ok || userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "未登录"})
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	var post model.CommunityPost
	if err := model.DB.First(&post, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "帖子不存在"})
		return
	}
	if post.UserID != userID || post.Status == model.CommunityPostStatusDeleted {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "msg": "无权限"})
		return
	}

	var req communityPostUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	req.Content = strings.TrimSpace(req.Content)
	if req.Title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "标题不能为空"})
		return
	}
	if req.Content == "" && len(req.Images) == 0 && strings.TrimSpace(req.VideoURL) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "内容不能为空"})
		return
	}

	images := req.Images
	if images == nil {
		images = []string{}
	}
	imagesJSON, _ := json.Marshal(images)
	updates := map[string]any{
		"title":           req.Title,
		"content":         req.Content,
		"images":          string(imagesJSON),
		"video_url":       strings.TrimSpace(req.VideoURL),
		"video_cover_url": strings.TrimSpace(req.VideoCoverURL),
		"status":          model.CommunityPostStatusPending,
		"reject_reason":   "",
	}
	if err := model.DB.Model(&model.CommunityPost{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "更新失败"})
		return
	}

	if err := model.DB.First(&post, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "更新失败"})
		return
	}
	nicknameMap, avatarMap := buildNicknameAvatarMap([]uint64{post.UserID})
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": toCommunityPostResponse(post, nicknameMap[post.UserID], avatarMap[post.UserID])})
}

// DeleteCommunityPost 教学端：删除帖子
func DeleteCommunityPost(c *gin.Context) {
	if model.DB == nil {
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "未连接数据库", "data": nil})
		return
	}
	userID, ok := currentUserID(c)
	if !ok || userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "未登录"})
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	var post model.CommunityPost
	if err := model.DB.First(&post, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "帖子不存在"})
		return
	}
	if post.UserID != userID || post.Status == model.CommunityPostStatusDeleted {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "msg": "无权限"})
		return
	}

	if err := model.DB.Model(&model.CommunityPost{}).Where("id = ?", id).Update("status", model.CommunityPostStatusDeleted).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "删除失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": true})
}

func toggleLike(c *gin.Context, liked bool) {
	if model.DB == nil {
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "未连接数据库", "data": nil})
		return
	}
	userID, ok := currentUserID(c)
	if !ok || userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "未登录"})
		return
	}
	postID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if postID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}

	var post model.CommunityPost
	if err := model.DB.First(&post, postID).Error; err != nil || post.Status == model.CommunityPostStatusDeleted {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "帖子不存在"})
		return
	}

	err := model.DB.Transaction(func(tx *gorm.DB) error {
		var existing model.CommunityPostLike
		findErr := tx.Where("user_id = ? AND post_id = ?", userID, postID).First(&existing).Error
		if liked {
			if findErr == nil {
				return nil
			}
			if !errors.Is(findErr, gorm.ErrRecordNotFound) {
				return findErr
			}
			if err := tx.Create(&model.CommunityPostLike{UserID: userID, PostID: postID}).Error; err != nil {
				return err
			}
			return tx.Model(&model.CommunityPost{}).Where("id = ?", postID).UpdateColumn("likes_count", gorm.Expr("likes_count + 1")).Error
		}
		if errors.Is(findErr, gorm.ErrRecordNotFound) {
			return nil
		}
		if findErr != nil {
			return findErr
		}
		if err := tx.Delete(&existing).Error; err != nil {
			return err
		}
		return tx.Model(&model.CommunityPost{}).Where("id = ? AND likes_count > 0", postID).UpdateColumn("likes_count", gorm.Expr("likes_count - 1")).Error
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "操作失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": true})
}

func toggleFavorite(c *gin.Context, favorited bool) {
	if model.DB == nil {
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "未连接数据库", "data": nil})
		return
	}
	userID, ok := currentUserID(c)
	if !ok || userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "未登录"})
		return
	}
	postID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if postID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}

	var post model.CommunityPost
	if err := model.DB.First(&post, postID).Error; err != nil || post.Status == model.CommunityPostStatusDeleted {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "帖子不存在"})
		return
	}

	err := model.DB.Transaction(func(tx *gorm.DB) error {
		var existing model.CommunityPostFavorite
		findErr := tx.Where("user_id = ? AND post_id = ?", userID, postID).First(&existing).Error
		if favorited {
			if findErr == nil {
				return nil
			}
			if !errors.Is(findErr, gorm.ErrRecordNotFound) {
				return findErr
			}
			if err := tx.Create(&model.CommunityPostFavorite{UserID: userID, PostID: postID}).Error; err != nil {
				return err
			}
			return tx.Model(&model.CommunityPost{}).Where("id = ?", postID).UpdateColumn("favorites_count", gorm.Expr("favorites_count + 1")).Error
		}
		if errors.Is(findErr, gorm.ErrRecordNotFound) {
			return nil
		}
		if findErr != nil {
			return findErr
		}
		if err := tx.Delete(&existing).Error; err != nil {
			return err
		}
		return tx.Model(&model.CommunityPost{}).Where("id = ? AND favorites_count > 0", postID).UpdateColumn("favorites_count", gorm.Expr("favorites_count - 1")).Error
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "操作失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": true})
}

func LikeCommunityPost(c *gin.Context) {
	toggleLike(c, true)
}

func UnlikeCommunityPost(c *gin.Context) {
	toggleLike(c, false)
}

func FavoriteCommunityPost(c *gin.Context) {
	toggleFavorite(c, true)
}

func UnfavoriteCommunityPost(c *gin.Context) {
	toggleFavorite(c, false)
}

type adminCommunityListResponse struct {
	List     []communityPostResponse `json:"list"`
	Total    int64                   `json:"total"`
	Page     int                     `json:"page"`
	PageSize int                     `json:"page_size"`
}

// AdminGetCommunityPosts 后台：帖子管理列表
func AdminGetCommunityPosts(c *gin.Context) {
	if !requireAdminAccount(c) {
		return
	}
	if model.DB == nil {
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "未连接数据库", "data": adminCommunityListResponse{List: []communityPostResponse{}, Total: 0, Page: 1, PageSize: 20}})
		return
	}

	page, pageSize := getPagination(c)
	keyword := strings.TrimSpace(c.Query("keyword"))
	status := strings.TrimSpace(c.Query("status"))

	query := model.DB.Model(&model.CommunityPost{})
	if status != "" && status != "all" {
		query = query.Where("status = ?", status)
	}
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("title LIKE ? OR content LIKE ?", like, like)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "获取帖子失败"})
		return
	}

	var posts []model.CommunityPost
	if err := query.Order("created_at desc").Limit(pageSize).Offset((page - 1) * pageSize).Find(&posts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "获取帖子失败"})
		return
	}

	nicknameMap, avatarMap := buildNicknameAvatarMap(extractUserIDs(posts))
	resp := make([]communityPostResponse, 0, len(posts))
	for _, item := range posts {
		resp = append(resp, toCommunityPostResponse(item, nicknameMap[item.UserID], avatarMap[item.UserID]))
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": adminCommunityListResponse{List: resp, Total: total, Page: page, PageSize: pageSize}})
}

// AdminGetCommunityPostDetail 后台：帖子详情
func AdminGetCommunityPostDetail(c *gin.Context) {
	if !requireAdminAccount(c) {
		return
	}
	if model.DB == nil {
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "未连接数据库", "data": nil})
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	var post model.CommunityPost
	if err := model.DB.First(&post, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "msg": "帖子不存在"})
		return
	}
	nicknameMap, avatarMap := buildNicknameAvatarMap([]uint64{post.UserID})
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": toCommunityPostResponse(post, nicknameMap[post.UserID], avatarMap[post.UserID])})
}

// AdminApproveCommunityPost 后台：审核通过
func AdminApproveCommunityPost(c *gin.Context) {
	if !requireAdminAccount(c) {
		return
	}
	if model.DB == nil {
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "未连接数据库", "data": nil})
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	updates := map[string]any{"status": model.CommunityPostStatusApproved, "reject_reason": ""}
	if err := model.DB.Model(&model.CommunityPost{}).Where("id = ? AND status <> ?", id, model.CommunityPostStatusDeleted).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "操作失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": true})
}

type adminRejectRequest struct {
	Reason string `json:"reason"`
}

// AdminRejectCommunityPost 后台：驳回
func AdminRejectCommunityPost(c *gin.Context) {
	if !requireAdminAccount(c) {
		return
	}
	if model.DB == nil {
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "未连接数据库", "data": nil})
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	var req adminRejectRequest
	_ = c.ShouldBindJSON(&req)
	req.Reason = strings.TrimSpace(req.Reason)
	if req.Reason == "" {
		req.Reason = "内容不符合规范"
	}
	updates := map[string]any{"status": model.CommunityPostStatusRejected, "reject_reason": req.Reason}
	if err := model.DB.Model(&model.CommunityPost{}).Where("id = ? AND status <> ?", id, model.CommunityPostStatusDeleted).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "操作失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": true})
}

type adminFlagsRequest struct {
	IsPinned   *int8 `json:"is_pinned"`
	IsFeatured *int8 `json:"is_featured"`
}

// AdminUpdateCommunityPostFlags 后台：置顶/精华
func AdminUpdateCommunityPostFlags(c *gin.Context) {
	if !requireAdminAccount(c) {
		return
	}
	if model.DB == nil {
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "未连接数据库", "data": nil})
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	var req adminFlagsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	updates := map[string]any{}
	if req.IsPinned != nil {
		updates["is_pinned"] = *req.IsPinned
	}
	if req.IsFeatured != nil {
		updates["is_featured"] = *req.IsFeatured
	}
	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	if err := model.DB.Model(&model.CommunityPost{}).Where("id = ? AND status <> ?", id, model.CommunityPostStatusDeleted).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "操作失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": true})
}

// AdminDeleteCommunityPost 后台：删除/下架
func AdminDeleteCommunityPost(c *gin.Context) {
	if !requireAdminAccount(c) {
		return
	}
	if model.DB == nil {
		c.JSON(http.StatusOK, gin.H{"code": 200, "msg": "未连接数据库", "data": nil})
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "参数错误"})
		return
	}
	if err := model.DB.Model(&model.CommunityPost{}).Where("id = ?", id).Update("status", model.CommunityPostStatusDeleted).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "删除失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": true})
}
