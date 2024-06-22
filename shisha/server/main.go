// main.go
package main

import (
	"context"
	"fmt"
	"os"
	"server/internal/controllers"
	"server/internal/database"
	"server/internal/initializers"
	"server/internal/models"
	"time"

	"github.com/gofiber/contrib/fiberzerolog"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/rs/zerolog"
	"github.com/urfave/cli/v2"
)

var (
	Version = "dev"
	Ctx     = context.Background()
)

func beforeAction(c *cli.Context) error {

	zerolog.MessageFieldName = "msg"
	databaseURL := c.String("database-url")
	redisAddr := c.String("redis-addr")
	s3Endpoint := c.String("s3-endpoint")
	s3AccessKey := c.String("s3-access-key")
	s3SecretKey := c.String("s3-secret-key")
	// DB
	err := database.Connect(databaseURL)
	if err != nil {
		return err
	}
	err = database.Migrate(&models.User{}, &models.Image{}, &models.PremiumImage{}, &models.Purchase{})

	if err != nil {
		return err
	}
	// RedPanda

	topic := c.String("redpanda-topic")
	brokers := c.StringSlice("redpanda-url")
	admin, err := initializers.NewAdmin(brokers)
	if err != nil {
		return err
	}
	defer admin.Close()

	topicExists, err := admin.TopicExists(Ctx, topic)
	if err != nil {
		return err
	}
	if !topicExists {
		err = admin.CreateTopic(Ctx, topic)
		if err != nil {
			return err
		}
	}

	// Redis
	err = initializers.InitRedis(Ctx, redisAddr)
	if err != nil {
		return err
	}

	// Minio
	err = initializers.InitMinIO(Ctx, s3Endpoint, s3AccessKey, s3SecretKey)
	if err != nil {
		return err
	}
	return nil
}

func mainAction(c *cli.Context) error {
	listenAddr := c.String("listen")
	jwtKey := c.String("secret-key")

	topic := c.String("redpanda-topic")
	brokers := c.StringSlice("redpanda-url")
	uploadController := controllers.NewUploadController(database.DB, initializers.MinioClient, initializers.Rdb, Ctx)
	err := uploadController.HandleBulkDownload()
	if err != nil {
		return err
	}

	imageController := controllers.NewImageController(database.DB, initializers.MinioClient, Ctx)

	// Back
	router := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})
	logger := zerolog.New(os.Stderr).With().Timestamp().Logger()

	router.Use(fiberzerolog.New(fiberzerolog.Config{
		Logger: &logger,
	}))
	router.Use(cors.New())

	router.Post("/api/register", controllers.Register)
	router.Post("/api/login", controllers.Login(jwtKey))
	router.Post("/api/transfer", controllers.Tranfser(topic, brokers, Ctx))
	router.Get("/api/balance", controllers.Balance)
	router.Post("/api/upload", controllers.AuthRequired, uploadController.HandleUpload)
	router.Get("/prem-images", imageController.GetPremiumImages)
	router.Get("/user-images", imageController.GetUserImages)
	router.Post("/purchase", imageController.PurchaseImage(topic, brokers))
	router.Get("/purchased/:userName", imageController.GetPurchasedImages)
	router.Get("/purchased/ids/:userName", imageController.GetPurchasedImageIDs)
	router.Get("/prem-images/url/:imageUUID", imageController.GetMinioURLOfPremiumImageByUUID)

	return router.Listen(listenAddr)
}
func main() {

	app := &cli.App{
		Name:     "shisha-inventory",
		Usage:    "shisha-inventory",
		Before:   beforeAction,
		Action:   mainAction,
		Version:  Version,
		Compiled: time.Now(),
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "log-level",
				Usage:   "logger verbosity `LEVEL`",
				Value:   "info",
				EnvVars: []string{"SHISHA_LOG_LEVEL"},
			},

			&cli.StringFlag{
				Name:    "listen",
				Usage:   "listen addr",
				Value:   "0.0.0.0:8080",
				EnvVars: []string{"SHISHA_LISTEN"},
			},

			&cli.StringFlag{
				Name:    "database-url",
				Usage:   "database url",
				Value:   "postgres://postgres:yourpassword@localhost:5432/shishaDB?sslmode=disable",
				EnvVars: []string{"SHISHA_DATABASE_URL"},
			},
			&cli.StringFlag{
				Name:    "secret-key",
				Usage:   "secret key",
				Value:   "your_secret_key",
				EnvVars: []string{"SHISHA_SECRET_KEY"},
			},
			&cli.StringSliceFlag{
				Name:    "redpanda-url",
				Usage:   "redpand url",
				Value:   cli.NewStringSlice("localhost:9092"),
				EnvVars: []string{"SHISHA_REDPANDA_URL"},
			},

			&cli.StringFlag{
				Name:    "redpanda-topic",
				Usage:   "redpand topic",
				Value:   "shisha",
				EnvVars: []string{"SHISHA_REDPANDA_TOPIC"},
			},

			&cli.StringFlag{
				Name:     "s3-endpoint",
				Usage:    "s3 endpoint",
				Required: true,
				EnvVars:  []string{"SHISHA_S3_ENDPOINT"},
			},
			&cli.StringFlag{
				Name:    "s3-access-key",
				Usage:   "s3 access key",
				Value:   "root",
				EnvVars: []string{"SHISHA_S3_ACCESS_KEY"},
			},

			&cli.StringFlag{
				Name:    "s3-secret-key",
				Usage:   "s3 secret key",
				Value:   "rootroot",
				EnvVars: []string{"SHISHA_S3_SECRET_KEY"},
			},

			&cli.StringFlag{
				Name:    "redis-addr",
				Usage:   "redis address",
				Value:   "localhost:6379",
				EnvVars: []string{"SHISHA_REDIS_ADDR"},
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
