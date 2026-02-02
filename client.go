package s3client

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"time"

	"codeberg.org/mkchar/s3client/utils"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

type Client struct {
	s3Client   *s3.Client
	uploader   *manager.Uploader
	downloader *manager.Downloader
	cfg        Config
}

func New(cfg Config) (*Client, error) {
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}
	awsCfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID, cfg.SecretAccessKey, "")),
		config.WithRegion(cfg.Region),
	)
	if err != nil {
		return nil, err
	}

	s3Client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.Region = cfg.Region
		o.BaseEndpoint = aws.String(cfg.Endpoint)
		o.UsePathStyle = true
	})

	return &Client{
		s3Client:   s3Client,
		uploader:   manager.NewUploader(s3Client),
		downloader: manager.NewDownloader(s3Client),
		cfg:        cfg,
	}, nil
}

func (c *Client) CreateBucket(ctx context.Context, name string) error {
	_, err := c.s3Client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(name),
	})
	return err
}

func (c *Client) DeleteBucket(ctx context.Context, name string) error {
	_, err := c.s3Client.DeleteBucket(ctx, &s3.DeleteBucketInput{
		Bucket: aws.String(name),
	})
	return err
}

func (c *Client) BucketExists(ctx context.Context, name string) (bool, error) {
	_, err := c.s3Client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(name),
	})
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) && apiErr.ErrorCode() == "NotFound" {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (c *Client) WaitBucketExists(ctx context.Context, name string, timeout time.Duration) error {
	waiter := s3.NewBucketExistsWaiter(c.s3Client)
	return waiter.Wait(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(name),
	}, timeout)
}

func (c *Client) ListBuckets(ctx context.Context) ([]string, error) {
	resp, err := c.s3Client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return nil, err
	}
	var buckets []string
	for _, bucket := range resp.Buckets {
		if bucket.Name != nil {
			buckets = append(buckets, *bucket.Name)
		}
	}
	return buckets, nil
}

func (c *Client) PutObject(ctx context.Context, bucket, key string, body io.Reader, contentType string) error {
	_, err := c.s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        body,
		ContentType: aws.String(contentType),
	})
	return err
}

func (c *Client) PutObjectBytes(ctx context.Context, bucket, key string, data []byte, contentType string) error {
	return c.PutObject(ctx, bucket, key, bytes.NewReader(data), contentType)
}

func (c *Client) GetObject(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
	output, err := c.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	return output.Body, nil
}

func (c *Client) GetObjectBytes(ctx context.Context, bucket, key string) ([]byte, error) {
	obj, err := c.GetObject(ctx, bucket, key)
	if err != nil {
		return nil, err
	}
	defer obj.Close()
	return io.ReadAll(obj)
}

func (c *Client) DeleteObject(ctx context.Context, bucket, key string) error {
	_, err := c.s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	return err
}

func (c *Client) ObjectExists(ctx context.Context, bucket, key string) (bool, error) {
	_, err := c.s3Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) && apiErr.ErrorCode() == "NotFound" {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (c *Client) ListObjects(ctx context.Context, bucket, prefix string) ([]string, error) {
	resp, err := c.s3Client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		return nil, err
	}
	var keys []string
	for _, obj := range resp.Contents {
		if obj.Key != nil {
			keys = append(keys, *obj.Key)
		}
	}
	return keys, nil
}

func (c *Client) DeleteObjects(ctx context.Context, bucket string, keys []string) error {
	var deleteObjects []types.ObjectIdentifier
	for _, key := range keys {
		deleteObjects = append(deleteObjects, types.ObjectIdentifier{Key: aws.String(key)})
	}

	_, err := c.s3Client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
		Bucket: aws.String(bucket),
		Delete: &types.Delete{Objects: deleteObjects},
	})
	return err
}

func (c *Client) UploadFile(ctx context.Context, bucket, key, localPath string) error {
	file, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = c.uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(key),
		Body:        file,
		ContentType: aws.String(utils.DetectContentType(path.Ext(localPath))),
	})
	return err
}

func (c *Client) DownloadFile(ctx context.Context, bucket, key, localPath string) error {
	file, err := os.Create(localPath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = c.downloader.Download(ctx, file, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	return err
}

func (c *Client) CopyObject(ctx context.Context, srcBucket, srcKey, dstBucket, dstKey string) error {
	_, err := c.s3Client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(dstBucket),
		CopySource: aws.String(fmt.Sprintf("%s/%s", srcBucket, srcKey)),
		Key:        aws.String(dstKey),
	})
	return err
}

func (c *Client) MoveObject(ctx context.Context, bucket, srcKey, dstKey string) error {
	if err := c.CopyObject(ctx, bucket, srcKey, bucket, dstKey); err != nil {
		return err
	}
	return c.DeleteObject(ctx, bucket, srcKey)
}

func (c *Client) PresignGetObject(ctx context.Context, bucket, key string, expiry time.Duration) (string, error) {
	presigner := s3.NewPresignClient(c.s3Client)
	presignReq, err := presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = expiry
	})
	if err != nil {
		return "", err
	}
	return presignReq.URL, nil
}

func (c *Client) PresignPutObject(ctx context.Context, bucket, key string, expiry time.Duration) (string, error) {
	presigner := s3.NewPresignClient(c.s3Client)
	presignReq, err := presigner.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = expiry
	})
	if err != nil {
		return "", err
	}
	return presignReq.URL, nil
}

func (c *Client) EmptyBucket(ctx context.Context, bucket string) error {
	objects, err := c.ListObjects(ctx, bucket, "")
	if err != nil {
		return err
	}
	if len(objects) > 0 {
		return c.DeleteObjects(ctx, bucket, objects)
	}
	return nil
}
