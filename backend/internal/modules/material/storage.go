package material

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"streetlight/internal/apperr"
)

// FileStorage 负责现场照片/视频在本地磁盘上的存取。
// 对象存储可通过实现同名方法替换, 业务层只依赖 storageKey。
type FileStorage struct {
	root string
}

// NewFileStorage 构造本地文件存储, root 为媒体文件根目录。
func NewFileStorage(root string) (*FileStorage, error) {
	cleaned := filepath.Clean(strings.TrimSpace(root))
	if cleaned == "" || cleaned == "." {
		return nil, fmt.Errorf("媒体文件目录不能为空")
	}
	if err := os.MkdirAll(cleaned, 0o755); err != nil {
		return nil, fmt.Errorf("创建媒体文件目录失败: %w", err)
	}
	return &FileStorage{root: cleaned}, nil
}

// Save 将上传内容写入磁盘, 返回存储键(相对路径)。
func (s *FileStorage) Save(reader io.Reader, ext string) (string, error) {
	shard, name, err := randomKey()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(s.root, shard)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("创建媒体分片目录失败: %w", err)
	}
	key := filepath.Join(shard, name+ext)
	target := filepath.Join(s.root, key)

	file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", fmt.Errorf("创建媒体文件失败: %w", err)
	}
	if _, err := io.Copy(file, reader); err != nil {
		_ = file.Close()
		_ = os.Remove(target)
		return "", fmt.Errorf("写入媒体文件失败: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(target)
		return "", fmt.Errorf("保存媒体文件失败: %w", err)
	}
	return filepath.ToSlash(key), nil
}

// Open 打开指定存储键对应的文件, 调用方负责关闭。
func (s *FileStorage) Open(key string) (*os.File, error) {
	path, err := s.safePath(key)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, apperr.NotFound("媒体文件已丢失或被清理: %s", key)
		}
		return nil, fmt.Errorf("读取媒体文件失败: %w", err)
	}
	return file, nil
}

// Remove 删除指定存储键对应的文件, 文件不存在时不视为错误。
func (s *FileStorage) Remove(key string) error {
	path, err := s.safePath(key)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除媒体文件失败: %w", err)
	}
	return nil
}

// safePath 校验存储键, 防止目录穿越。
func (s *FileStorage) safePath(key string) (string, error) {
	key = filepath.Clean(strings.TrimSpace(key))
	if key == "" || key == "." || strings.HasPrefix(key, "..") || filepath.IsAbs(key) {
		return "", apperr.NotFound("非法的媒体存储键: %s", key)
	}
	return filepath.Join(s.root, key), nil
}

// randomKey 生成两级随机分片目录与随机文件名。
func randomKey() (shard, name string, err error) {
	buf := make([]byte, 18)
	if _, err = rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("生成媒体文件名失败: %w", err)
	}
	encoded := hex.EncodeToString(buf)
	return encoded[:2], encoded[2:], nil
}
