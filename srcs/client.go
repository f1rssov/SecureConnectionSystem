package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"os"

	"golang.org/x/crypto/ssh"
)

func startClient() {
	// Загружаем приватный ключ клиента
	keyBytes, err := os.ReadFile("./keys/id_rsa")
	if err != nil {
		log.Fatal("Ошибка чтения приватного ключа клиента:", err)
	}
	signer, err := ssh.ParsePrivateKey(keyBytes)
	if err != nil {
		log.Fatal("Ошибка парсинга приватного ключа клиента:", err)
	}

	// Загружаем публичный ключ сервера для проверки HostKeyCallback
	// Предполагается, что это файл ./keys/server.key.pub
	hostKeyBytes, err := os.ReadFile("./keys/server.key.pub")
	if err != nil {
		log.Fatal("Не удалось прочитать публичный ключ сервера (./keys/server.key.pub):", err)
	}
	// Парсим как authorized key, так как это стандартный формат для одиночного публичного ключа
	hostPublicKey, _, _, _, err := ssh.ParseAuthorizedKey(hostKeyBytes)
	if err != nil {
		log.Fatal("Не удалось распарсить публичный ключ сервера:", err)
	}

	config := &ssh.ClientConfig{
		User: "user", // Этот пользователь должен быть известен серверу, если PublicKeyCallback его проверяет
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		// HostKeyCallback: ssh.InsecureIgnoreHostKey(), // <-- ЗАМЕНЕНО
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			log.Printf("Проверка ключа хоста для %s (%s). Тип ключа: %s", hostname, remote, key.Type())
			if bytes.Equal(key.Marshal(), hostPublicKey.Marshal()) {
				log.Println("Публичный ключ сервера совпадает с ожидаемым.")
				return nil // Ключ хоста совпадает
			}
			log.Println("ВНИМАНИЕ: Публичный ключ сервера НЕ СОВПАДАЕТ с ожидаемым! Атака MITM?")
			return fmt.Errorf("публичный ключ сервера не совпадает с ожидаемым")
		},
	}

	// Устанавливаем соединение с сервером
	log.Println("Подключение к SSH серверу 127.0.0.1:2222...")
	conn, err := ssh.Dial("tcp", "127.0.0.1:2222", config)
	if err != nil {
		log.Fatal("Не удалось подключиться к SSH серверу: ", err)
	}
	log.Println("SSH-соединение с сервером установлено.")

	// Запускаем локальный SOCKS5 прокси на 127.0.0.1:1080
	listener, err := net.Listen("tcp", "127.0.0.1:1080")
	if err != nil {
		log.Fatal("Не удалось запустить локальный прокси:", err)
	}
	log.Println("SOCKS5 прокси работает на 127.0.0.1:1080")

	for {
		clientConn, err := listener.Accept()
		if err != nil {
			log.Println("Ошибка подключения клиента:", err)
			continue
		}
		go handleSOCKS(clientConn, conn)
	}
}

// handleSOCKS (без изменений, как в вашем коде)
func handleSOCKS(client net.Conn, sshConn *ssh.Client) {
	defer client.Close()

	buf := make([]byte, 262)

	// Читаем приветствие от клиента
	n, err := client.Read(buf)
	if err != nil {
		log.Println("Ошибка чтения приветствия SOCKS5:", err)
		return
	}
	if n < 2 || buf[0] != 0x05 {
		log.Println("Неверный SOCKS5 формат")
		return
	}

	// Отправляем ответ — метод: без авторизации
	client.Write([]byte{0x05, 0x00})

	// Читаем второй пакет от клиента — куда подключиться
	n, err = client.Read(buf)
	if err != nil {
		log.Println("Ошибка чтения запроса SOCKS5:", err)
		return
	}
	if n < 7 {
		log.Println("Слишком короткий запрос SOCKS5")
		return
	}

	addrType := buf[3]
	var host string
	var port int

	switch addrType {
	case 0x01: // IPv4
		host = net.IP(buf[4:8]).String()
		port = int(buf[8])<<8 | int(buf[9])
	case 0x03: // доменное имя
		nameLen := int(buf[4])
		host = string(buf[5 : 5+nameLen])
		port = int(buf[5+nameLen])<<8 | int(buf[6+nameLen])
	default:
		log.Println("Неподдерживаемый тип адреса:", addrType)
		return
	}

	target := net.JoinHostPort(host, fmt.Sprint(port))
	log.Println("Проксируем через SSH на:", target)

	remote, err := sshConn.Dial("tcp", target)
	if err != nil {
		log.Println("Не удалось подключиться к цели через SSH:", err)
		// Сообщаем SOCKS клиенту об ошибке
		// Коды ответа SOCKS5:
		// 0x01: general SOCKS server failure
		// 0x03: Network unreachable
		// 0x04: Host unreachable
		// 0x05: Connection refused
		client.Write([]byte{0x05, 0x01, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
		return
	}
	defer remote.Close()

	// Успешный ответ клиенту
	// Адрес и порт здесь могут быть фиктивными, т.к. соединение уже установлено через туннель
	client.Write([]byte{0x05, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})

	// Прокидываем данные
	errChan := make(chan error, 2)
	go func() {
		_, err := io.Copy(remote, client)
		errChan <- err
	}()
	go func() {
		_, err := io.Copy(client, remote)
		errChan <- err
	}()

	// Ждем завершения одной из копирующих горутин или ошибки
	<-errChan
	log.Printf("Соединение с %s через SOCKS прокси завершено.", target)
}