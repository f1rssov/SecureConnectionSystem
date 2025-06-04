package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"time"

	"golang.org/x/crypto/ssh"
)

// Обёртка ssh.Channel под net.Conn
type sshConnWrapper struct {
	ssh.Channel
	localAddr  net.Addr
	remoteAddr net.Addr
}

func (s *sshConnWrapper) LocalAddr() net.Addr {
	return s.localAddr
}

func (s *sshConnWrapper) RemoteAddr() net.Addr {
	return s.remoteAddr
}

func (s *sshConnWrapper) SetDeadline(t time.Time) error {
	return nil 
}

func (s *sshConnWrapper) SetReadDeadline(t time.Time) error {
	return nil 
}

func (s *sshConnWrapper) SetWriteDeadline(t time.Time) error {
	return nil 
}

// Заглушка для адресов 
type dummyAddr string

func (d dummyAddr) Network() string { return string(d) }
func (d dummyAddr) String() string  { return string(d) }

func startServ() {
	// Загружаем приватный ключ сервера
	privateBytes, err := os.ReadFile("./keys/server.key")
	if err != nil {
		log.Fatal("Не удалось прочитать приватный ключ сервера:", err)
	}

	private, err := ssh.ParsePrivateKey(privateBytes)
	if err != nil {
		log.Fatal("Не удалось распарсить приватный ключ сервера:", err)
	}

	// Загружаем авторизованный публичный ключ клиента
	authorizedKeyBytes, err := os.ReadFile("./keys/id_rsa.pub")
	if err != nil {
		log.Fatal("Не удалось прочитать авторизованный публичный ключ клиента (./keys/id_rsa.pub):", err)
	}
	authorizedKey, _, _, _, err := ssh.ParseAuthorizedKey(authorizedKeyBytes)
	if err != nil {
		log.Fatal("Не удалось распарсить авторизованный публичный ключ клиента:", err)
	}

	// Конфиг SSH-сервера с аутентификацией по публичному ключу
	config := &ssh.ServerConfig{
		// NoClientAuth: true, // <-- УБРАНО: теперь требуется аутентификация клиента
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			log.Printf("Клиент %s (%s) пытается аутентифицироваться с ключом типа %s", conn.User(), conn.RemoteAddr(), key.Type())
			// Сравниваем предоставленный клиентом ключ с авторизованным ключом
			if bytes.Equal(key.Marshal(), authorizedKey.Marshal()) {
				log.Printf("Клиент %s авторизован по публичному ключу.", conn.User())
				return nil, nil // Успешная аутентификация
			}
			log.Printf("Клиент %s: предоставленный публичный ключ не авторизован.", conn.User())
			return nil, fmt.Errorf("публичный ключ отклонен для пользователя %s", conn.User())
		},
	}
	config.AddHostKey(private)

	// Слушаем порт 2222
	listener, err := net.Listen("tcp", "127.0.0.1:2222")
	if err != nil {
		log.Fatal("Ошибка при старте сервера:", err)
	}
	log.Println("SSH сервер запущен на 127.0.0.1:2222 и требует аутентификации клиента")

	for {
		nConn, err := listener.Accept()
		if err != nil {
			log.Println("Ошибка соединения:", err)
			continue
		}
		go handleConnection(nConn, config)
	}
}

func handleConnection(nConn net.Conn, config *ssh.ServerConfig) {
	sshConn, chans, reqs, err := ssh.NewServerConn(nConn, config)
	if err != nil {
		// Ошибка может возникнуть здесь, если клиент не прошел аутентификацию
		log.Printf("Ошибка SSH соединения (возможно, аутентификация не удалась): %v", err)
		return
	}
	defer sshConn.Close()
	log.Printf("Новое SSH-соединение от %s (пользователь: %s)", sshConn.RemoteAddr(), sshConn.User())

	go ssh.DiscardRequests(reqs)

	for newChannel := range chans {
		switch newChannel.ChannelType() {
		case "direct-tcpip":
			go handleDirectTCPIP(newChannel)
		default:
			newChannel.Reject(ssh.UnknownChannelType, "Неподдерживаемый тип канала")
		}
	}
}

func handleDirectTCPIP(newChannel ssh.NewChannel) {
	var channelData struct {
		DestAddr   string
		DestPort   uint32
		OriginAddr string
		OriginPort uint32
	}

	if err := ssh.Unmarshal(newChannel.ExtraData(), &channelData); err != nil {
		log.Println("Ошибка разбора данных канала:", err)
		newChannel.Reject(ssh.Prohibited, "Некорректные данные")
		return
	}

	dest := fmt.Sprintf("%s:%d", channelData.DestAddr, channelData.DestPort)
	log.Printf("Запрос подключения к %s", dest)

	targetConn, err := net.Dial("tcp", dest)
	if err != nil {
		log.Printf("Не удалось подключиться к %s: %v", dest, err)
		newChannel.Reject(ssh.ConnectionFailed, err.Error())
		return
	}

	channel, requests, err := newChannel.Accept()
	if err != nil {
		log.Println("Ошибка при приёме канала:", err)
		targetConn.Close()
		return
	}
	go ssh.DiscardRequests(requests)

	go func() {
		defer channel.Close()
		defer targetConn.Close()
		io.Copy(channel, targetConn)
	}()

	go func() {
		defer channel.Close()
		defer targetConn.Close()
		io.Copy(targetConn, channel)
	}()
}