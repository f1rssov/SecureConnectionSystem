#Генерирование ключей для сервера
#ssh-keygen -t rsa -b 2048 -f server.key

#kill -9 $(lsof -ti :2222) освобождает порт

#ssh -p 2222 user@localhost -N -D 1080 подключение
#make run IP_PORT=

#ssh -R 12345:127.0.0.1:2222 serveo.net
#nohup ./secure_tunnel_server > server.log 2>&1 &
#tail -f server.log
#ssh -l firssov 158.160.90.225

# Пути к ключам
KEY_DIR=./keys
SERVER_KEY=$(KEY_DIR)/server.key
SERVER_PUB=$(KEY_DIR)/server.key.pub
CLIENT_KEY=$(KEY_DIR)/id_rsa
CLIENT_PUB=$(KEY_DIR)/id_rsa.pub

# Docker-serv
DOCKER_IMAGE_NAME=secure_ssh_server
DOCKER_CONTAINER_NAME=secure_ssh_demonstration


# Переменная для IP:PORT SSH-сервера, по умолчанию используется локальный Docker
# Можно переопределить при вызове make: make run IP_PORT=0.tcp.ngrok.io:12345
IP_PORT ?= 127.0.0.1:2222

# Основной бинарник
BINARY=secure_tunnel
#IP для  доступа к удаленному серверу
YANDEX_IP =firssov@158.160.90.225
#IP для запуска SSH сервера на удаленном сервере
YANDEX_SERV_IP =158.160.90.225:2222

# Порт сервера
PORT=2222
SOCKS_PORT=1080

# ==== Команды ====
#Для запуска  локального SSH  сервера для тестов
all: run
# 1. Генерация SSH-ключей сервера и клиента
keys:
	mkdir -p $(KEY_DIR)
	ssh-keygen -t rsa -b 2048 -f $(SERVER_KEY) -N "" -q
	ssh-keygen -t rsa -b 2048 -f $(CLIENT_KEY) -N "" -q
	chmod 600 $(SERVER_KEY) $(CLIENT_KEY)
	@echo "Ключи успешно сгенерированы."

# 2. Очистка всех ключей и временных файлов
clean:
	@rm -rf $(KEY_DIR) $(BINARY)
	@echo "Очищено: ключи, бинарник, временные профили."

fclean: clean docker-delete
	@rm -rf $(BINARY)

# 3. Убить процесс, занимающий порт 2222
free-port:
	@echo "Освобождаем порт $(PORT)..."
	@kill -9 $$(lsof -ti :$(PORT)) 2>/dev/null || true
	@kill -9 $$(lsof -ti :$(SOCKS_PORT)) 2>/dev/null || true

# 4. Сборка Go проекта
build:
	@echo "Сборка..."
	go build -o $(BINARY) ./srcs/main.go ./srcs/server.go ./srcs/client.go ./srcs/conf_proxy.go
	@echo "Готово: $(BINARY)"

# 5. Запуск (сначала генерирует ключи, билдит, освобождает порт и запускает)
run: free-port keys build
	@echo "Запуск приложения..."
	./$(BINARY)
	@rm -rf $(KEY_DIR)

# 6. Подключение вручную через SSH-клиент(для тестов, лучше не трогать)
connect:
	@echo "Подключение к SSH серверу на порту 2222..."
	ssh -p $(PORT) user@localhost -N -D 1080

# 7. Полный цикл: очистка, генерация, билд и запуск(все локальное)
rebuild: clean run



#---------------------------------------------------------------------------

#---------------------------------------------------------------------------



# Сборка Docker образа (Для демонстрации подключениz к якобы "удаленному" серверу)
#Используем ldflags для внедрения значения IP_PORT (make docker-auto IP_PORT=<ip_нашего_сервера>:<port>)
docker-auto: free-port keys docker-build docker-stop docker-run docker-connect
	./$(BINARY)

docker-build:
	@echo "Сборка Docker образа $(DOCKER_IMAGE_NAME)..."
	docker build -t $(DOCKER_IMAGE_NAME) .
	@echo "Docker образ собран."

# Запуск Docker контейнера
docker-run:
	@echo "Запуск Docker контейнера $(DOCKER_CONTAINER_NAME) на порту $(PORT)..."
	docker run -d --rm --name $(DOCKER_CONTAINER_NAME) -p $(PORT):2222 $(DOCKER_IMAGE_NAME)
	@echo "Docker контейнер запущен. SSH доступен на 127.0.0.1:$(PORT)"

# Остановка Docker контейнера
docker-stop:
	@echo "Остановка Docker контейнера $(DOCKER_CONTAINER_NAME)..."
	docker stop $(DOCKER_CONTAINER_NAME) 2>/dev/null || true
	@echo "Docker контейнер остановлен."

docker-delete: docker-stop
	docker rmi $(DOCKER_IMAGE_NAME):latest

docker-rebuild: free-port keys docker-stop docker-build docker-run

# Перезапуск контейнера
docker-restart: docker-stop docker-run

# Используем ldflags для внедрения значения IP_PORT (make docker-auto IP_PORT=<ip_нашего_сервера>:<port>)
docker-connect:
	@echo "Сборка Go-клиента с IP_PORT=$(IP_PORT)..."
	go build -o $(BINARY) -ldflags="-X 'main.sshServerAddr=$(IP_PORT)'" ./demo-srcs/main.go ./demo-srcs/client.go ./demo-srcs/conf_proxy.go
	@echo "Готово: $(BINARY)"



#---------------------------------------------------------------------------

#---------------------------------------------------------------------------



#Предпологается что у пользователя уже есть доступ к полноценному удаленному серверу
#Используем ldflags для внедрения значения YANDEX_SERV_IP (make docker-auto IP_PORT=<ip_нашего_сервера>:<port>)
#Копируются все нужные файлы для запуска сервера и осущ. сам запуск сервера
yandex-server-start: free-port keys
	@echo "Сборка Go-клиента с IP_PORT=$(YANDEX_SERV_IP)..."
	go build -o $(BINARY) -ldflags="-X 'main.sshServerAddr=$(YANDEX_SERV_IP)'" ./demo-srcs/main.go ./demo-srcs/client.go ./demo-srcs/conf_proxy.go
	@echo "Готово: $(BINARY)"
	ssh -i ~/.ssh/id_rsa $(YANDEX_IP) -T "mkdir -p /home/firssov/keys"
	scp -i ~/.ssh/id_rsa ./makefile $(YANDEX_IP):/home/firssov/
	scp -i ~/.ssh/id_rsa ./keys/server.key $(YANDEX_IP):/home/firssov/keys
	scp -i ~/.ssh/id_rsa ./keys/server.key.pub $(YANDEX_IP):/home/firssov/keys
	scp -i ~/.ssh/id_rsa ./keys/id_rsa $(YANDEX_IP):/home/firssov/keys
	scp -i ~/.ssh/id_rsa ./keys/id_rsa.pub $(YANDEX_IP):/home/firssov/keys
	scp -i ~/.ssh/id_rsa ./demo-srcs/server/secure_tunnel_server $(YANDEX_IP):/home/firssov/
	ssh -i ~/.ssh/id_rsa $(YANDEX_IP) -T "nohup ./secure_tunnel_server > server.log 2>&1 &"

#Для ручного запуска сервера
serv-on:
	ssh -i ~/.ssh/id_rsa $(YANDEX_IP) -T "nohup ./secure_tunnel_server > server.log 2>&1 &"

#Подключение к серверу
yandex-start-connection:
	./$(BINARY)

yandex-delete-files: clean free-port
	ssh -i ~/.ssh/id_rsa $(YANDEX_IP) -T "rm -rf secure_tunnel_server makefile ./keys server.log"
