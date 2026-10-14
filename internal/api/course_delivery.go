package api

import (
	"errors"
	"net/http"
	"strings"
	"time"
	"ydxt-go/internal/model"
	"ydxt-go/internal/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func deliveryReply(c *gin.Context, data interface{}, err error) {
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "success", "data": data})
}

func deliveryDB(c *gin.Context) bool {
	if model.DB == nil {
		c.JSON(503, gin.H{"code": 503, "msg": "数据库未连接"})
		return false
	}
	return true
}

func deliveryIdentity(c *gin.Context) *utils.CustomClaims {
	raw := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
	if raw == "" {
		raw = c.GetHeader("X-Token")
	}
	claims, err := utils.ParseToken(raw)
	if err != nil {
		return nil
	}
	return claims
}

func deliveryStaff(c *gin.Context, courseID uint64) bool {
	claims := deliveryIdentity(c)
	if claims == nil {
		return false
	}
	if claims.Role == 9 {
		return true
	}
	var course model.Course
	return claims.Role == 2 && model.DB.First(&course, courseID).Error == nil && course.TeacherID == claims.TeacherID
}

func GetDeliverySettings(c *gin.Context) {
	if !deliveryDB(c) {
		return
	}
	id, ok := parseUint(c, "id")
	if !ok {
		return
	}
	var setting model.CourseDeliverySetting
	err := model.DB.First(&setting, "course_id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		setting.CourseID = id
		err = nil
	}
	hasAccess := false
	if claims := deliveryIdentity(c); claims != nil && err == nil {
		hasAccess = coursePurchaseAccess(id, 0, claims.UserID) == nil
	}
	deliveryReply(c, struct {
		model.CourseDeliverySetting
		HasAccess bool `json:"has_access"`
	}{setting, hasAccess}, err)
}

func SaveDeliverySettings(c *gin.Context) {
	if !deliveryDB(c) {
		return
	}
	id, ok := parseUint(c, "id")
	if !ok {
		return
	}
	course, ok := ensureCoursePermission(c, id)
	if !ok {
		return
	}
	var setting model.CourseDeliverySetting
	if err := c.ShouldBindJSON(&setting); err != nil || setting.ValidityDays < 0 || setting.ValidityDays > 36500 {
		deliveryReply(c, nil, errors.New("有效期须为 0 到 36500 天"))
		return
	}
	setting.CourseID = id
	if course.Level == "class" || course.Level == "single" {
		setting.Sequential = false
	}
	deliveryReply(c, setting, model.DB.Save(&setting).Error)
}

func GetCourseCohorts(c *gin.Context) {
	if !deliveryDB(c) {
		return
	}
	id, ok := parseUint(c, "id")
	if !ok {
		return
	}
	query := model.DB.Where("course_id = ?", id)
	if !deliveryStaff(c, id) {
		claims := deliveryIdentity(c)
		if claims == nil {
			query = query.Where("status = ?", "published")
		} else {
			query = query.Where("status = 'published' OR id IN (SELECT cohort_id FROM course_enrollments WHERE user_id = ?)", claims.UserID)
		}
	}
	rows := []model.CourseCohort{}
	err := query.Order("start_at desc, id desc").Find(&rows).Error
	type cohortView struct {
		model.CourseCohort
		Enrolled int64 `json:"enrolled"`
		Joined   bool  `json:"joined"`
	}
	result := []cohortView{}
	claims := deliveryIdentity(c)
	for _, row := range rows {
		view := cohortView{CourseCohort: row}
		if err == nil {
			err = model.DB.Table("course_enrollments e").Joins("JOIN orders o ON o.id = e.order_id").
				Where("e.cohort_id = ? AND o.status IN ?", row.ID, []string{"paid", "shipped"}).Count(&view.Enrolled).Error
		}
		if claims != nil {
			var n int64
			model.DB.Table("course_enrollments e").Joins("JOIN orders o ON o.id = e.order_id").
				Where("e.cohort_id = ? AND e.user_id = ? AND o.status IN ? AND (e.expires_at IS NULL OR e.expires_at > ?)", row.ID, claims.UserID, []string{"paid", "shipped"}, time.Now()).Count(&n)
			view.Joined = n > 0
		}
		result = append(result, view)
	}
	deliveryReply(c, result, err)
}

func SaveCourseCohort(c *gin.Context) {
	if !deliveryDB(c) {
		return
	}
	id, ok := parseUint(c, "id")
	if !ok {
		return
	}
	course, ok := ensureCoursePermission(c, id)
	if !ok {
		return
	}
	if course.Level != "class" {
		deliveryReply(c, nil, errors.New("仅班课支持班期"))
		return
	}
	var req struct {
		ID                 uint64 `json:"id"`
		Name               string `json:"name"`
		Capacity           int    `json:"capacity"`
		Price              int    `json:"price"`
		EnrollmentDeadline string `json:"enrollment_deadline"`
		StartAt            string `json:"start_at"`
		EndAt              string `json:"end_at"`
		Status             string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		deliveryReply(c, nil, err)
		return
	}
	deadline, start, end := parseTime(req.EnrollmentDeadline), parseTime(req.StartAt), parseTime(req.EndAt)
	if strings.TrimSpace(req.Name) == "" || req.Capacity < 1 || req.Price < 0 || deadline == nil || start == nil || end == nil ||
		deadline.After(*start) || !end.After(*start) || (req.Status != "draft" && req.Status != "published" && req.Status != "closed") {
		deliveryReply(c, nil, errors.New("请检查班期名称、人数、价格、状态和时间范围"))
		return
	}
	row := model.CourseCohort{ID: req.ID, CourseID: id, Name: strings.TrimSpace(req.Name), Capacity: req.Capacity,
		Price: req.Price, EnrollmentDeadline: *deadline, StartAt: *start, EndAt: *end, Status: req.Status}
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		if row.ID == 0 {
			return tx.Create(&row).Error
		}
		var old model.CourseCohort
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&old, "id = ? AND course_id = ?", row.ID, id).Error; err != nil {
			return err
		}
		var n int64
		if err := tx.Table("course_enrollments e").Joins("JOIN orders o ON o.id = e.order_id").
			Where("e.cohort_id = ? AND o.status IN ?", row.ID, []string{"paid", "shipped"}).Count(&n).Error; err != nil {
			return err
		}
		if int64(row.Capacity) < n {
			return errors.New("人数上限不能低于已报名人数")
		}
		row.CreatedAt = old.CreatedAt
		return tx.Save(&row).Error
	})
	deliveryReply(c, row, err)
}

func GetCohortMembers(c *gin.Context) {
	if !deliveryDB(c) {
		return
	}
	id, ok := parseUint(c, "id")
	if !ok {
		return
	}
	var cohort model.CourseCohort
	if err := model.DB.First(&cohort, id).Error; err != nil {
		deliveryReply(c, nil, err)
		return
	}
	if _, ok := ensureCoursePermission(c, cohort.CourseID); !ok {
		return
	}
	type member struct {
		model.CourseEnrollment
		Nickname string `json:"nickname"`
		Attended int64  `json:"attended"`
	}
	result := []member{}
	err := model.DB.Table("course_enrollments e").Select("e.*, u.nickname, (SELECT COUNT(*) FROM course_attendances a JOIN course_lessons l ON l.id = a.lesson_id WHERE a.user_id = e.user_id AND l.cohort_id = e.cohort_id) AS attended").
		Joins("LEFT JOIN users u ON u.id = e.user_id").Where("e.cohort_id = ?", id).Scan(&result).Error
	deliveryReply(c, result, err)
}

// Enrollment is granted only in the payment transaction.
func enrollPaidCourses(tx *gorm.DB, order model.Order, items []model.OrderItem) error {
	for _, item := range items {
		if item.ItemType != "course" {
			continue
		}
		var course model.Course
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&course, item.GoodsID).Error; err != nil {
			return err
		}
		if course.Status != 1 {
			return errors.New("课程已下架")
		}
		if course.Level == "class" && item.CohortID == 0 {
			return errors.New("班课订单缺少班期，请重新下单")
		}
		var expires *time.Time
		if item.CohortID != 0 {
			var cohort model.CourseCohort
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&cohort, "id = ? AND course_id = ?", item.CohortID, course.ID).Error; err != nil {
				return err
			}
			if cohort.Status != "published" || time.Now().After(cohort.EnrollmentDeadline) {
				return errors.New("班期已停止报名")
			}
			var n int64
			if err := tx.Table("course_enrollments e").Joins("JOIN orders o ON o.id = e.order_id").
				Where("e.cohort_id = ? AND e.user_id <> ? AND o.status IN ?", cohort.ID, order.UserID, []string{"paid", "shipped"}).Count(&n).Error; err != nil {
				return err
			}
			if n >= int64(cohort.Capacity) {
				return errors.New("班期人数已满")
			}
		}
		var setting model.CourseDeliverySetting
		err := tx.First(&setting, "course_id = ?", course.ID).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if setting.ValidityDays > 0 {
			t := time.Now().AddDate(0, 0, setting.ValidityDays)
			expires = &t
		}
		row := model.CourseEnrollment{CourseID: course.ID, CohortID: item.CohortID, UserID: order.UserID, OrderID: order.ID, ExpiresAt: expires}
		var existing model.CourseEnrollment
		err = tx.First(&existing, "course_id = ? AND cohort_id = ? AND user_id = ?", row.CourseID, row.CohortID, row.UserID).Error
		if err == nil {
			var active int64
			if err := tx.Model(&model.Order{}).Where("id = ? AND user_id = ? AND status IN ?", existing.OrderID, order.UserID, []string{"paid", "shipped"}).Count(&active).Error; err != nil {
				return err
			}
			if active > 0 && (existing.ExpiresAt == nil || existing.ExpiresAt.After(time.Now())) {
				return errors.New("已开通该课程或班期，无需重复购买")
			}
			row.ID = existing.ID
			if err := tx.Save(&row).Error; err != nil {
				return err
			}
		} else if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		} else {
			return err
		}
	}
	return nil
}

func deleteDeliveryContent(courseID uint64, remove func(*gorm.DB) error) error {
	return model.DB.Transaction(func(tx *gorm.DB) error {
		var course model.Course
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&course, courseID).Error; err != nil {
			return err
		}
		var count int64
		if err := tx.Table("order_items i").Joins("JOIN orders o ON o.id = i.order_id").
			Where("i.item_type = 'course' AND i.goods_id = ? AND o.status IN ?", courseID, []string{"paid", "shipped"}).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return errors.New("课程已有已付款学员，不能删除授课内容；请使用下架或编辑")
		}
		if err := tx.Model(&model.CourseEnrollment{}).Where("course_id = ?", courseID).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return errors.New("课程已有报名记录，不能删除授课内容")
		}
		if tx.Migrator().HasTable("user_courses") {
			if err := tx.Table("user_courses").Where("course_id = ?", courseID).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return errors.New("课程已有历史学员授权，不能删除授课内容")
			}
		}
		return remove(tx)
	})
}

func CreateCourseOrder(c *gin.Context) {
	if !deliveryDB(c) {
		return
	}
	userID, ok := currentOrderUserID(c)
	if !ok {
		return
	}
	var req struct {
		CourseID uint64 `json:"course_id"`
		CohortID uint64 `json:"cohort_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		deliveryReply(c, nil, err)
		return
	}
	var course model.Course
	if err := model.DB.First(&course, "id = ? AND status = 1", req.CourseID).Error; err != nil {
		deliveryReply(c, nil, err)
		return
	}
	normalizeCoursePrice(&course)
	price := int(course.Price*100 + 0.5)
	if course.Level == "class" {
		var cohort model.CourseCohort
		if err := model.DB.First(&cohort, "id = ? AND course_id = ? AND status = 'published'", req.CohortID, course.ID).Error; err != nil {
			deliveryReply(c, nil, errors.New("请选择有效班期"))
			return
		}
		if time.Now().After(cohort.EnrollmentDeadline) {
			deliveryReply(c, nil, errors.New("班期已停止报名"))
			return
		}
		price = cohort.Price
	} else {
		req.CohortID = 0
	}
	order := model.Order{OrderNo: generateOrderNo(), UserID: userID, Type: "shop", Amount: price, PayAmount: price, Status: "unpaid", PayMethod: 1}
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&order).Error; err != nil {
			return err
		}
		return tx.Create(&model.OrderItem{OrderID: order.ID, ItemType: "course", GoodsID: course.ID, CohortID: req.CohortID, GoodsName: course.Title, Price: price, Quantity: 1}).Error
	})
	deliveryReply(c, gin.H{"id": order.ID, "order_no": order.OrderNo, "amount": float64(price) / 100}, err)
}

func coursePurchaseAccess(courseID, cohortID, userID uint64) error {
	var enrollment model.CourseEnrollment
	err := model.DB.First(&enrollment, "course_id = ? AND cohort_id = ? AND user_id = ?", courseID, cohortID, userID).Error
	if err == nil {
		if enrollment.ExpiresAt != nil && time.Now().After(*enrollment.ExpiresAt) {
			return errors.New("课程学习有效期已到期")
		}
		var order model.Order
		err := model.DB.First(&order, "id = ? AND user_id = ? AND status IN ?", enrollment.OrderID, userID, []string{"paid", "shipped"}).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("订单已失效")
		}
		return err
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if cohortID != 0 {
		return errors.New("请先报名该班期")
	}
	// Legacy paid orders use their original payment time.
	var order model.Order
	err = model.DB.Table("orders o").Select("o.*").Joins("JOIN order_items i ON o.id = i.order_id").
		Where("i.item_type = 'course' AND i.goods_id = ? AND o.user_id = ? AND o.status IN ?", courseID, userID, []string{"paid", "shipped"}).
		Order("o.pay_time desc").Take(&order).Error
	if err == nil {
		paidAt := order.PayTime
		if paidAt.IsZero() {
			paidAt = order.CreatedAt
		}
		var setting model.CourseDeliverySetting
		err := model.DB.First(&setting, "course_id = ?", courseID).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if setting.ValidityDays > 0 && time.Now().After(paidAt.AddDate(0, 0, setting.ValidityDays)) {
			return errors.New("课程学习有效期已到期")
		}
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if model.DB.Migrator().HasTable("user_courses") {
		var legacy struct{ ExpiredAt *time.Time }
		err := model.DB.Table("user_courses").Where("course_id = ? AND user_id = ?", courseID, userID).Take(&legacy).Error
		if err == nil {
			if legacy.ExpiredAt != nil && time.Now().After(*legacy.ExpiredAt) {
				return errors.New("课程学习有效期已到期")
			}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
	}
	return errors.New("请先购买课程")
}

func courseAccess(c *gin.Context, lesson model.CourseLesson) error {
	var chapter model.CourseChapter
	if err := model.DB.First(&chapter, lesson.ChapterID).Error; err != nil {
		return err
	}
	if deliveryStaff(c, chapter.CourseID) {
		return nil
	}
	if lesson.ExpireAt != nil && time.Now().After(*lesson.ExpireAt) {
		return errors.New("该课时或回放已过期")
	}
	if lesson.IsFree == 1 {
		return nil
	}
	claims := deliveryIdentity(c)
	if claims == nil {
		return errors.New("请先登录并购买课程")
	}
	if err := coursePurchaseAccess(chapter.CourseID, lesson.CohortID, claims.UserID); err != nil {
		return err
	}
	var setting model.CourseDeliverySetting
	if err := model.DB.First(&setting, "course_id = ?", chapter.CourseID).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if setting.Sequential && lesson.CohortID == 0 {
		var previous model.CourseLesson
		err := model.DB.Table("course_lessons l").Select("l.*").Joins("JOIN course_chapters ch ON ch.id = l.chapter_id").
			Where("ch.course_id = ? AND l.cohort_id = 0 AND (ch.sort_order < ? OR (ch.sort_order = ? AND ch.id < ?) OR (ch.id = ? AND (l.sort_order < ? OR (l.sort_order = ? AND l.id < ?))))",
				chapter.CourseID, chapter.SortOrder, chapter.SortOrder, chapter.ID, chapter.ID, lesson.SortOrder, lesson.SortOrder, lesson.ID).
			Order("ch.sort_order desc, ch.id desc, l.sort_order desc, l.id desc").Take(&previous).Error
		if err == nil {
			var count int64
			if err := model.DB.Model(&model.UserLessonProgress{}).Where("user_id = ? AND lesson_id = ? AND is_completed = 1", claims.UserID, previous.ID).Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				return errors.New("请先完成上一课时")
			}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
	}
	return nil
}

func GetDeliveryLesson(c *gin.Context) {
	if !deliveryDB(c) {
		return
	}
	id, ok := parseUint(c, "id")
	if !ok {
		return
	}
	var lesson model.CourseLesson
	if err := model.DB.First(&lesson, id).Error; err != nil {
		deliveryReply(c, nil, err)
		return
	}
	if err := courseAccess(c, lesson); err != nil {
		deliveryReply(c, nil, err)
		return
	}
	if lesson.Type == 3 {
		if lesson.LiveStatus == 1 {
			claims := deliveryIdentity(c)
			if claims != nil {
				err := model.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&model.CourseAttendance{LessonID: id, UserID: claims.UserID}).Error
				if err != nil {
					deliveryReply(c, nil, err)
					return
				}
			}
		} else if !(lesson.LiveStatus == 2 && lesson.ReplayEnabled && lesson.ReplayStatus == "ready" && lesson.ReplayFileID != "") {
			deliveryReply(c, nil, errors.New("尚未开播或回放尚未就绪"))
			return
		}
	}
	deliveryReply(c, lesson, nil)
}

// Single courses and cohort schedules use one internal container chapter.
func EnsureDeliveryChapter(c *gin.Context) {
	if !deliveryDB(c) {
		return
	}
	id, ok := parseUint(c, "id")
	if !ok {
		return
	}
	if _, ok := ensureCoursePermission(c, id); !ok {
		return
	}
	var chapter model.CourseChapter
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		var course model.Course
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&course, id).Error; err != nil {
			return err
		}
		if course.Level != "single" && course.Level != "class" {
			return errors.New("该类型请使用章节管理")
		}
		if course.Level == "single" {
			var n int64
			if err := tx.Table("course_lessons l").Joins("JOIN course_chapters ch ON ch.id = l.chapter_id").Where("ch.course_id = ?", id).Count(&n).Error; err != nil {
				return err
			}
			if n > 1 {
				return errors.New("该单课已有多个课时，请先整理课程目录")
			}
		}
		err := tx.Where("course_id = ?", id).Order("sort_order,id").First(&chapter).Error
		if err == nil {
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		chapter = model.CourseChapter{CourseID: id, Title: "课程内容", SortOrder: 1}
		return tx.Create(&chapter).Error
	})
	deliveryReply(c, chapter, err)
}

func GetCohortLessons(c *gin.Context) {
	if !deliveryDB(c) {
		return
	}
	id, ok := parseUint(c, "id")
	if !ok {
		return
	}
	var cohort model.CourseCohort
	if err := model.DB.First(&cohort, id).Error; err != nil {
		deliveryReply(c, nil, err)
		return
	}
	if cohort.Status != "published" && !deliveryStaff(c, cohort.CourseID) {
		claims := deliveryIdentity(c)
		var count int64
		if claims != nil {
			model.DB.Model(&model.CourseEnrollment{}).Where("cohort_id = ? AND user_id = ?", id, claims.UserID).Count(&count)
		}
		if count == 0 {
			deliveryReply(c, nil, errors.New("班期尚未发布"))
			return
		}
	}
	lessons := []model.CourseLesson{}
	err := model.DB.Where("cohort_id = ?", id).Order("COALESCE(live_start, created_at), sort_order, id").Find(&lessons).Error
	redactDeliveryLessons(c, cohort.CourseID, lessons)
	deliveryReply(c, lessons, err)
}

func GetDeliveryProgress(c *gin.Context) {
	if !deliveryDB(c) {
		return
	}
	id, ok := parseUint(c, "id")
	if !ok {
		return
	}
	userID, ok := currentOrderUserID(c)
	if !ok {
		return
	}
	var lesson model.CourseLesson
	if err := model.DB.First(&lesson, id).Error; err != nil {
		deliveryReply(c, nil, err)
		return
	}
	if err := courseAccess(c, lesson); err != nil {
		deliveryReply(c, nil, err)
		return
	}
	var progress model.UserLessonProgress
	err := model.DB.First(&progress, "lesson_id = ? AND user_id = ?", id, userID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = nil
	}
	deliveryReply(c, progress, err)
}

func validateCourseLevelChange(id uint64, level string) error {
	if level == "single" {
		var n int64
		if err := model.DB.Table("course_lessons l").Joins("JOIN course_chapters ch ON ch.id = l.chapter_id").Where("ch.course_id = ?", id).Count(&n).Error; err != nil {
			return err
		}
		if n > 1 {
			return errors.New("已有多个课时，不能改为单课")
		}
	}
	if level != "class" {
		var n int64
		if err := model.DB.Model(&model.CourseCohort{}).Where("course_id = ?", id).Count(&n).Error; err != nil {
			return err
		}
		if n > 0 {
			return errors.New("已设置班期的课程不能更换为其他类型")
		}
	}
	return nil
}

func SetDeliveryLiveStatus(c *gin.Context) {
	if !deliveryDB(c) {
		return
	}
	id, ok := parseUint(c, "id")
	if !ok {
		return
	}
	var lesson model.CourseLesson
	if err := model.DB.First(&lesson, id).Error; err != nil {
		deliveryReply(c, nil, err)
		return
	}
	var chapter model.CourseChapter
	if err := model.DB.First(&chapter, lesson.ChapterID).Error; err != nil {
		deliveryReply(c, nil, err)
		return
	}
	if _, ok := ensureCoursePermission(c, chapter.CourseID); !ok {
		return
	}
	var req struct {
		Status int8 `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || lesson.Type != 3 || (req.Status != 1 && req.Status != 2) {
		deliveryReply(c, nil, errors.New("无效的直播状态"))
		return
	}
	if req.Status == 1 && (lesson.LiveStatus == 2 || lesson.MediaURL == "") {
		deliveryReply(c, nil, errors.New("已结束的直播不能重新开播，且须配置直播播放地址"))
		return
	}
	if req.Status == 2 && lesson.LiveStatus != 1 {
		deliveryReply(c, nil, errors.New("直播尚未开始"))
		return
	}
	deliveryReply(c, nil, model.DB.Model(&lesson).Update("live_status", req.Status).Error)
}
