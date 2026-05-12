package config

import (
	"os"
	"strconv"
	"strings"
)

func Env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func EnvInt(key string, def int) int {
	v, err := strconv.Atoi(Env(key, ""))
	if err != nil {
		return def
	}
	return v
}

func EnvBool(key string, def bool) bool {
	s := Env(key, "")
	if s == "" {
		return def
	}
	return s == "true" || s == "1" || s == "yes"
}

func EnvList(key, def, sep string) []string {
	return strings.Split(Env(key, def), sep)
}
