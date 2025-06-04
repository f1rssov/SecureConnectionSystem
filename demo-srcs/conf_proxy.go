package main

import (
	"log"
	"os/exec"
	"path/filepath"
	"os"
)

// Путь до Chrome на macOS
const chromePath = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"

// Флаг прокси
const proxyFlag = "--proxy-server=socks5://127.0.0.1:1080"

// Создание отдельного пользовательского профиля (опционально)
var chromeProfileDir = filepath.Join(os.TempDir(), "secure_proxy_profile")

// Запуск Chrome с прокси
func startChromeWithProxy() {
	log.Println("Запуск Chrome с SOCKS5 прокси...")

	cmd := exec.Command(
		chromePath,
		proxyFlag,
		"--user-data-dir="+chromeProfileDir, // изолированный профиль, чтобы не мешать основному
		"--no-first-run",
		"--no-default-browser-check",
	)

	err := cmd.Start()
	if err != nil {
		log.Fatalf("Не удалось запустить Chrome: %v", err)
	}
	log.Println("Chrome запущен с прокси.")
}
