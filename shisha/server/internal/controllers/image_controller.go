package controllers

import (
	"context"
	"net/http"
	"net/url"
	"server/internal/initializers"
	"server/internal/models"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jinzhu/gorm"
	"github.com/minio/minio-go/v7"
)

type ImageController struct {
	DB          *gorm.DB
	MinioClient *minio.Client
	Ctx         context.Context
}

func NewImageController(db *gorm.DB, minioClient *minio.Client, ctx context.Context) *ImageController {
	return &ImageController{
		DB:          db,
		MinioClient: minioClient,
		Ctx:         ctx,
	}
}

func (ic *ImageController) GetUserImages(c *fiber.Ctx) error {
	var images []models.Image
	if err := ic.DB.Find(&images).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch images"})
	}

	var imageList []fiber.Map
	for _, image := range images {
		reqParams := make(url.Values)
		presignedURL, err := ic.MinioClient.PresignedGetObject(ic.Ctx, "user-images", image.Name, time.Hour*24, reqParams)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to get image URL"})
		}
		imageList = append(imageList, fiber.Map{
			"id":         image.ID,
			"name":       image.Name,
			"uploadedAt": image.UploadedAt,
			"url":        presignedURL.String(),
			"owner":      image.Username,
		})
	}

	return c.JSON(imageList)
}

func (ic *ImageController) GetPremiumImages(c *fiber.Ctx) error {
	var images []models.PremiumImage
	if err := ic.DB.Find(&images).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch images"})
	}

	var imageList []fiber.Map
	for _, image := range images {
		reqParams := make(url.Values)
		presignedURL, err := ic.MinioClient.PresignedGetObject(ic.Ctx, "premium-images", image.UUID+".jpg", time.Hour*24, reqParams)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to get image URL"})
		}
		imageList = append(imageList, fiber.Map{
			"id":         image.ID,
			"name":       image.Name,
			"uploadedAt": image.UploadedAt,
			"url":        presignedURL.String(),
			"price":      image.Price,
		})
	}

	return c.JSON(imageList)
}

func (ic *ImageController) PurchaseImage(topic string, brokers []string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		type PurchaseRequest struct {
			ImageID  uint   `json:"image_id"`
			UserName string `json:"user_name"`
		}

		var request PurchaseRequest
		if err := c.BodyParser(&request); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request"})
		}

		var purchase models.Purchase
		if err := ic.DB.Where("user_name = ? AND image_id = ?", request.UserName, request.ImageID).First(&purchase).Error; err == nil {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "You have already purchased this image"})
		}

		var image models.PremiumImage
		if err := ic.DB.First(&image, "id = ?", request.ImageID).Error; err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Image not found"})
		}

		var user models.User
		if err := ic.DB.Where("username = ?", request.UserName).First(&user).Error; err != nil {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
		}

		if user.Coins < 25 {
			return c.Status(fiber.StatusPaymentRequired).JSON(fiber.Map{"error": "Insufficient coins"})
		}

		user.Coins -= 25
		if err := ic.DB.Save(&user).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update user balance"})
		}

		purchase = models.Purchase{
			UserName:  request.UserName,
			ImageID:   request.ImageID,
			ImageUUID: image.UUID,
			ImageName: image.Name,
		}
		if err := ic.DB.Create(&purchase).Error; err != nil {
			return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create purchase record"})
		}

		producer, err := initializers.NewProducer(brokers, topic)
		if err != nil {
			return err
		}
		// defer producer.Close()
		producer.SendBuyMessage(ic.Ctx, user.Username, image.UUID, 25)

		return c.JSON(fiber.Map{"message": "Image purchased successfully"})
	}
}

func (ic *ImageController) GetPurchasedImages(c *fiber.Ctx) error {
	userName := c.Params("userName")

	if userName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "User ID is required"})
	}

	var purchases []models.Purchase
	if err := ic.DB.Where("user_name = ?", userName).Find(&purchases).Error; err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch purchases"})
	}

	var imageList []fiber.Map
	for _, image := range purchases {
		reqParams := make(url.Values)
		presignedURL, err := ic.MinioClient.PresignedGetObject(ic.Ctx, "premium-images", image.ImageUUID+".jpg", time.Hour*24, reqParams)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to get image URL"})
		}
		imageList = append(imageList, fiber.Map{
			"id":      image.ID,
			"name":    image.ImageName,
			"url":     presignedURL.String(),
			"buytime": image.CreatedAt,
		})
	}

	return c.JSON(imageList)
}

func (ic *ImageController) GetPurchasedImageIDs(c *fiber.Ctx) error {
	userName := c.Params("userName")
	if userName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "UserName is required"})
	}

	var purchasedImages []models.Purchase

	if err := ic.DB.Where("user_name = ?", userName).Select("image_id").Find(&purchasedImages).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve purchased image IDs",
		})
	}

	imageIDs := make([]string, len(purchasedImages))
	for i, image := range purchasedImages {
		imageIDs[i] = strconv.FormatUint(uint64(image.ImageID), 10)
	}

	return c.JSON(imageIDs)
}

func (ic *ImageController) GetMinioURLOfPremiumImageByUUID(c *fiber.Ctx) error {
	imageUUID := c.Params("imageUUID")
	if imageUUID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "UserName is required"})
	}

	var image models.PremiumImage
	if err := ic.DB.Where("uuid = ?", imageUUID).First(&image).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Image not found"})
	}

	reqParams := make(url.Values)
	presignedURL, err := ic.MinioClient.PresignedGetObject(ic.Ctx, "premium-images", image.UUID+".jpg", time.Hour*24, reqParams)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to get image URL"})
	}

	var imageURL = fiber.Map{"url": presignedURL.String()}
	return c.JSON(imageURL)
}
