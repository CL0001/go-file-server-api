package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var db *gorm.DB

type File struct {
	ID           string    `gorm:"primaryKey"`
	Path         string
	OriginalName string
	CreatedBy    string
	CreatedAt    time.Time
}

func connect(host, user, password, dbName string) error {
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=5432 sslmode=disable TimeZone=Europe/Bratislava",
		host, user, password, dbName,
	)

	var err error
	db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return errors.New("cannot connect to database: " + err.Error())
	}

	if err := db.AutoMigrate(&File{}); err != nil {
		return errors.New("auto migration failed: " + err.Error())
	}

	return nil
}

func createFileRecord(id, originalName string) error {
	file := File{
		ID:           id,
		Path:         "app/uploads/" + id,
		OriginalName: originalName,
		CreatedBy:    "admin",
		CreatedAt:    time.Now(),
	}

	if err := db.Create(&file).Error; err != nil {
		return errors.New("record was not created: " + err.Error())
	}

	return nil
}

func checkUploads() error {
	_, err := os.Stat("uploads")

	if os.IsNotExist(err) {
		err := os.MkdirAll("uploads", os.ModePerm)
		if err != nil {
			return fmt.Errorf("failed to create directory: %v", err)
		}
		return nil
	} else if err != nil {
		return fmt.Errorf("error checking directory: %v", err)
	}
	return nil
}

func uploadFile(c echo.Context) error {
	file, err := c.FormFile("newFile")
	if err != nil {
		return err
	}

	src, err := file.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	id := uuid.NewString()

	err = checkUploads()
	if err != nil {
		return c.NoContent(http.StatusInternalServerError)
	}

	dst, err := os.Create(fmt.Sprintf("uploads/%s.csv", id))
	if err != nil {
		return err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return err
	}

	if err := createFileRecord(id, file.Filename); err != nil {
		return err
	}

	return c.String(http.StatusOK, id)
}

func getFileRecord(id string) (*File, error) {
	var file File
	if err := db.First(&file, "id = ?", id).Error; err != nil {
		return nil, errors.New("cannot retrieve record")
	}
	return &file, nil
}

func downloadFile(c echo.Context) error {
	fileRecord, err := getFileRecord(c.Param("id"))
	if err != nil {
		return c.String(http.StatusNotFound, fmt.Sprintf("Record with ID %s not found", c.Param("id")))
	}

	return c.File(fmt.Sprintf("./uploads/%s.csv", fileRecord.ID))
}

func showAllFiles(c echo.Context) error {
	var files []File
	db.Find(&files)

	return c.JSON(http.StatusOK, files)
}

func main() {
	if err := godotenv.Load(".env"); err != nil {
		panic(errors.New("cannot load .env file"))
	}

	if err := connect(os.Getenv("HOST"), os.Getenv("USER"), os.Getenv("PASSWD"), os.Getenv("DBNAME")); err != nil {
		panic(err)
	}

	app := echo.New()

	app.GET("/", func(c echo.Context) error {
		return c.File("index.html")
	})

	app.POST("/upload", uploadFile)

	app.GET("/download/:id", downloadFile)

	app.GET("/database", showAllFiles)

	app.Logger.Fatal(app.Start(":8000"))
}