# S3 sdk.
## AWS
## MinIOSDK
## RustFS

```bash
go get github.com/mkchar/s3client
```

```golang
	obj, err := s3client.New(s3client.Config{
		Endpoint:        "http://localhost:9000",
		AccessKeyID:     "UWCusA1LSEtaP5ZNRqQm",
		SecretAccessKey: "lG7qaSXe6DjVQJ3yUI9WmwLfAikBsTd18oxZHKYc",
		// Region:          "us-east-1",
	})
	if err != nil {
		fmt.Println(err)
	}
    bucketName := "Demo" 
	ctx := context.Background()
	err = obj.CreateBucket(ctx,bucketName)
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println("上传文件...")
	file, err := os.Open("./credentials.json")
	if err != nil {
		log.Fatalf("打开文件失败: %v", err)
	}
	defer file.Close()

	key := "uploads/example.txt" // rustfs 中的对象路径
	if err := obj.PutObject(ctx, bucketName, key, file, "text/json"); err != nil {
		log.Fatalf("上传失败: %v", err)
	}
	fmt.Printf("文件上传成功: %s/%s\n", bucketName, key)

	fmt.Println("验证文件...")
	downloaded, err := obj.GetObjectBytes(ctx, bucketName, key)
	if err != nil {
		log.Fatalf("下载验证失败: %v", err)
	}
	fmt.Printf("下载验证: %d 字节\n", len(downloaded))

=	fmt.Println("列出 Bucket 内容...")
	objects, err := obj.ListObjects(ctx, bucketName, "uploads/")
	if err != nil {
		log.Fatalf("列出失败: %v", err)
	}
	for _, obj := range objects {
		fmt.Println("  -", obj)
	}

```