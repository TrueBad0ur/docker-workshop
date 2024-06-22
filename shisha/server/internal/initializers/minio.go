package initializers

import (
	"context"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	zlog "github.com/rs/zerolog/log"
)

var MinioClient *minio.Client

func InitMinIO(ctx context.Context, endpoint string, accessKey string, secretKey string) error {
	var err error
	MinioClient, err = minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: false,
	})
	if err != nil {
		return err
	}

	err = MinioClient.MakeBucket(ctx, "premium-images", minio.MakeBucketOptions{})
	if err != nil {
		exists, errBucketExists := MinioClient.BucketExists(ctx, "premium-images")
		if errBucketExists == nil && exists {
			zlog.Print("We already own premium-images")
		} else {
			return err
		}
	} else {
		zlog.Print("Successfully created premium-images")
	}

	err = MinioClient.MakeBucket(ctx, "user-images", minio.MakeBucketOptions{})
	if err != nil {
		exists, errBucketExists := MinioClient.BucketExists(ctx, "user-images")
		if errBucketExists == nil && exists {
			zlog.Print("We already own user-images")
		} else {
			return err
		}
	} else {
		zlog.Print("Successfully created user-images")
	}
	return nil
}
