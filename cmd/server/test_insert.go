package main

import (
	"fmt"
	"yuedi_edu/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func main() {
	db, err := gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	db.AutoMigrate(&model.Course{})
	
	course := model.Course{
		Title: "Test",
	}
	err = db.Create(&course).Error
	fmt.Println("Error:", err)
	fmt.Println("ID:", course.ID)
}
