// Package account 负责账号身份、状态和登录态，不分配 GameServer。
package account

import (
	"errors"
	"regexp"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrBadRequest  = errors.New("invalid account request")
	ErrInvalid     = errors.New("invalid credentials or session")
	ErrExpired     = errors.New("session credential expired")
	ErrUnavailable = errors.New("account name unavailable")
	accountPattern = regexp.MustCompile(`^[a-z0-9_-]{3,64}$`)
)

// credentials 规范化账号名，密码保持原字节，不截断 bcrypt 超长输入。
func credentials(name, password string) (string, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	if !accountPattern.MatchString(name) || len(password) < 12 || len(password) > 72 {
		return "", ErrBadRequest
	}
	return name, nil
}

// hashPassword 使用 bcrypt 标准库实现，工作因子固定为 10。
func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}
