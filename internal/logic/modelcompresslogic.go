// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package logic

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"toolbox/internal/config"
	"toolbox/internal/svc"
	"toolbox/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// 生成随机文件名（避免冲突）
func generateRandomFilename(ext string) string {
	b := make([]byte, 16)
	rand.Read(b)
	fileName := strings.ToLower("compress_" + base64.URLEncoding.EncodeToString(b)[:16])
	return fileName + ext
}

// 下载GLB文件到临时目录
func downloadGLBFromURL(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("invalid status code: %d", resp.StatusCode)
	}

	fileExt := filepath.Ext(url)
	if fileExt != ".glb" && fileExt != ".gltf" {
		return "", fmt.Errorf("unsupported file type: %s", fileExt)
	}

	tempFile, err := os.CreateTemp("", fmt.Sprintf("glb_temp_*%s", fileExt))
	if err != nil {
		return "", fmt.Errorf("create temp file failed: %w", err)
	}
	defer tempFile.Close()

	if _, err := io.Copy(tempFile, resp.Body); err != nil {
		return "", fmt.Errorf("write to temp file failed: %w", err)
	}

	return tempFile.Name(), nil
}

// 调用Rust优化工具
func callRustOptimizer(rustBinPath, tempInputPath, outputPath string, texSize uint, removeNormal, convertKTX2 bool) error {
	args := []string{
		"--input", tempInputPath,
		"--output", outputPath,
		"--tex-size", fmt.Sprintf("%d", texSize),
	}

	if removeNormal {
		args = append(args, "--remove-normal")
	}
	if convertKTX2 {
		args = append(args, "--convert-ktx2")
	}

	cmd := exec.Command(rustBinPath, args...)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start command: %w", err)
	}

	stderrBytes, _ := io.ReadAll(stderr)
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("command failed: %w\nstderr: %s", err, string(stderrBytes))
	}

	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		return fmt.Errorf("output file not generated")
	}

	return nil
}

// 上传文件到Supabase
func uploadToSupabase(cfg *config.Config, localPath string) (string, error) {
	file, err := os.Open(localPath)
	if err != nil {
		return "", fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	// 获取文件信息
	fileInfo, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("get file info: %w", err)
	}

	// 构建Supabase存储API URL
	fileName := generateRandomFilename(filepath.Ext(fileInfo.Name()))
	uploadURL := fmt.Sprintf("%s/storage/v1/object/%s/model/%s", cfg.Supabase.Url, cfg.Supabase.Bucket, fileName)

	// 创建HTTP请求
	req, err := http.NewRequest("POST", uploadURL, file)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	// 设置请求头
	req.Header.Set("Authorization", "Bearer "+cfg.Supabase.AnonKey)
	if cfg.Supabase.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.Supabase.AuthToken)
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("X-Upload-Content-Length", fmt.Sprintf("%d", fileInfo.Size()))

	// 发送请求
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("upload failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("upload failed: status %d, body: %s", resp.StatusCode, string(body))
	}

	uploadFileUrl := fmt.Sprintf("%s/storage/v1/object/public/%s/%s", cfg.Supabase.Url, cfg.Supabase.Bucket, fileName)
	fmt.Printf("upload file url: %s\n", uploadFileUrl)

	// 生成可访问的URL
	return uploadFileUrl, nil
}

// 修正后（正确）
func NewModelCompressHandlerLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ModelCompressHandlerLogic {
	return &ModelCompressHandlerLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// 定义逻辑结构体（必须）
type ModelCompressHandlerLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

// 主逻辑：整合下载、优化、上传流程
func (l *ModelCompressHandlerLogic) Handle(req *types.ModelCompressRequest, svcCtx *svc.ServiceContext) (*types.ModelCompressResponse, error) {
	logx.Infof("Received compress request: %+v", req)

	// 1. 下载原始文件到临时目录
	tempInputPath, err := downloadGLBFromURL(req.InputUrl)
	errMsg := fmt.Sprintf("下载文件失败: %v", err)
	if err != nil {
		logx.Error("Download failed:", err)
		return &types.ModelCompressResponse{
			Success: false,
			Error:   &errMsg,
		}, nil
	}
	defer os.Remove(tempInputPath) // 清理临时文件

	// 2. 创建优化后的临时输出文件
	tempOutputPath := filepath.Join(os.TempDir(), generateRandomFilename(".glb"))
	defer os.Remove(tempOutputPath) // 清理临时文件

	fmt.Printf("tempInputPath: %v \n", tempInputPath)
	fmt.Printf("tempOutputPath: %v \n", tempOutputPath)

	// 3. 调用Rust优化工具
	if err := callRustOptimizer(
		svcCtx.Config.RustBinPath,
		tempInputPath,
		tempOutputPath,
		req.TexSize,
		req.RemoveNormal,
		req.ConvertKtx2,
	); err != nil {
		logx.Error("Optimization failed:", err)
		errMsg := fmt.Sprintf("执行Rust命令失败: %v", err)
		return &types.ModelCompressResponse{
			Success: false,
			Error:   &errMsg,
		}, nil
	}

	// 4. 上传优化后的文件到Supabase
	supabaseURL, err := uploadToSupabase(&svcCtx.Config, tempOutputPath)
	if err != nil {
		logx.Error("Upload to Supabase failed:", err)
		errMsg := fmt.Sprintf("下载文件失败: %v", err)
		return &types.ModelCompressResponse{
			Success: false,
			Error:   &errMsg,
		}, nil
	}

	// 5. 返回成功结果
	return &types.ModelCompressResponse{
		Data:    types.CompressData{Url: supabaseURL}, // 通过 Data 字段嵌套 Url
		Error:   nil,
		Success: true,
	}, nil
}
