package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/junara/jheader-proxy/internal/config"
)

// readHeaderFiles は --header-file で指定された各ファイルを読み込み、ヘッダーの
// 一覧へ変換する。トークン等の秘匿値をシェル履歴や ps 出力に残さずに渡すための
// 経路なので、値の解釈は --header と完全に揃える(trim/重複解決は変換時に行う)。
func readHeaderFiles(paths []string) ([]config.HeaderKV, error) {
	var headers []config.HeaderKV
	for _, path := range paths {
		kvs, err := readHeaderFile(path)
		if err != nil {
			return nil, err
		}
		headers = append(headers, kvs...)
	}
	return headers, nil
}

// readHeaderFile は1つのヘッダーファイルを解析する。書式は1行1件の Name=Value。
// 空行と # 始まりの行(コメント)は無視する。
func readHeaderFile(path string) ([]config.HeaderKV, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read header file %q: %w", path, err)
	}
	var headers []config.HeaderKV
	for i, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		name, value, found := strings.Cut(trimmed, "=")
		if !found || strings.TrimSpace(name) == "" {
			return nil, fmt.Errorf("invalid header at %s:%d: must be Name=Value", path, i+1)
		}
		headers = append(headers, config.HeaderKV{Name: name, Value: value})
	}
	return headers, nil
}
