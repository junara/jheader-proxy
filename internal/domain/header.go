package domain

import (
	"fmt"
	"strings"
)

// Headers は対象リクエストに付与するHTTPヘッダーの順序付き集合。
// ゼロ値は空のまま利用可能。
type Headers struct {
	values map[string]string
	order  []string
}

// ParseHeaders は "Name=Value" 形式の指定を順序付きの Headers に変換する。
//
// ルール:
//   - 最初の "=" で分割する。"=" を含まない指定はエラー
//   - 名前は前後の空白を除去し、空であってはならない
//   - 値も前後の空白を除去する。空の値は許可する
//   - 同名が重複した場合は後勝ちとし、初出の位置を保持する
func ParseHeaders(specs []string) (Headers, error) {
	h := Headers{values: make(map[string]string, len(specs))}
	for _, spec := range specs {
		name, value, found := strings.Cut(spec, "=")
		if !found {
			return Headers{}, fmt.Errorf("invalid --header %q: must be Name=Value", spec)
		}
		name = strings.TrimSpace(name)
		value = strings.TrimSpace(value)
		if name == "" {
			return Headers{}, fmt.Errorf("invalid --header %q: header name is empty", spec)
		}
		if _, exists := h.values[name]; !exists {
			h.order = append(h.order, name)
		}
		h.values[name] = value
	}
	return h, nil
}

// IsSensitiveHeader は、ログ等で値を秘匿すべき機密ヘッダー名かを返す
// （大文字小文字は区別しない）。
func IsSensitiveHeader(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "authorization", "proxy-authorization", "cookie", "set-cookie", "x-api-key":
		return true
	default:
		return false
	}
}

// Len は重複を除いたヘッダー数を返す。
func (h Headers) Len() int { return len(h.values) }

// Each は初出順に各ヘッダーへ fn を呼び出す。
func (h Headers) Each(fn func(name, value string)) {
	for _, name := range h.order {
		fn(name, h.values[name])
	}
}
