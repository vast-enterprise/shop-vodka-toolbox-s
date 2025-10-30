// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package config

import "github.com/zeromicro/go-zero/rest"

type Config struct {
	rest.RestConf

	// 补充Rust可执行文件路径
	RustBinPath string `json:",default=./rust_bin/gltf_opt"`

	// Supabase配置
	Supabase struct {
		Url       string `json:",env=SUPABASE_URL"`
		AnonKey   string `json:",env=SUPABASE_ANON_KEY"`
		Bucket    string `json:",env=SUPABASE_BUCKET"`
		AuthToken string `json:",env=SUPABASE_AUTH_TOKEN"`
	}
}
