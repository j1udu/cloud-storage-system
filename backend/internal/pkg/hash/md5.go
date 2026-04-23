package hash

import (
	"crypto/md5"
	"encoding/hex"
	"io"
)

// MD5FromReader 从 Reader 计算文件的 MD5 哈希
func MD5FromReader(r io.Reader) (string, error) {
	h := md5.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
