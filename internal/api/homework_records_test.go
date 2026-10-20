package api

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"ydxt-go/internal/model"
)

func TestStudentHomeworkRecordsScopeAndFeedback(t *testing.T) {
	teacherTestRouter(t)
	model.DB.Model(&model.HomeworkSubmission{}).Where("id = 3").Updates(map[string]any{
		"feedback": "Keep practicing", "is_passed": 1,
	})
	mustCreate(t, model.DB, &model.HomeworkSubmission{ID: 10, UserID: 5, TaskID: 1, Status: "graded", Feedback: "Private"})
	router := gin.New()
	router.GET("/records", func(c *gin.Context) {
		c.Set("userID", uint64(1))
		GetMyHomeworkSubmissions(c)
	})
	for _, test := range []struct {
		URL   string
		Total int
	}{
		{"/records", 3}, {"/records?task_id=1", 2}, {"/records?task_id=99", 0},
	} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest("GET", test.URL, nil))
		var body struct {
			Data []struct {
				model.HomeworkSubmission
				TaskTitle   string `json:"task_title"`
				CourseTitle string `json:"course_title"`
				TotalScore  int    `json:"total_score"`
			} `json:"data"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if response.Code != 200 || len(body.Data) != test.Total {
			t.Fatalf("records query failed: %s", response.Body.String())
		}
		for _, item := range body.Data {
			if item.UserID != 1 || item.TaskTitle == "" || item.CourseTitle == "" {
				t.Fatalf("records lost identity scope or titles: %+v", item)
			}
			if item.ID == 3 && (item.Feedback != "Keep practicing" || item.Score != 60 || item.IsPassed != 1 || item.TotalScore != 80) {
				t.Fatalf("teacher review missing: %+v", item)
			}
		}
	}
}
