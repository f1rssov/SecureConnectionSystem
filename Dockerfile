# --- Этап сборки ---
# Используем официальный образ Go для компиляции приложения
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Копируем все необходимые исходные файлы Go
# (main.go, server.go, client.go, conf_proxy.go - даже если client/conf_proxy не используются сервером,
# их наличие может быть необходимо для компиляции main.go, если он их импортирует)
COPY ./demo-srcs/server/main.go .
COPY ./demo-srcs/server/server.go .
COPY ./demo-srcs/server/go.mod .
COPY ./demo-srcs/server/go.sum .

# Копируем директорию с ключами, необходимыми для работы Go-сервера
COPY ./keys ./keys/

# Модифицируем main.go, чтобы он запускал ТОЛЬКО startServ()
# Это гарантирует, что в контейнере будет работать только серверная част

# Компилируем Go-приложение
# CGO_ENABLED=0 делает бинарник статически слинкованным, что позволяет использовать его в минимальном образе
# GOOS=linux указывает целевую ОС
RUN CGO_ENABLED=0 GOOS=linux go build -o secure_go_server .

# --- Этап запуска ---
# Используем легковесный образ Alpine для запуска скомпилированного приложения
FROM alpine:3.18

WORKDIR /app

# Копируем скомпилированный бинарник из предыдущего этапа
COPY --from=builder /app/secure_go_server .

# Копируем ключи, необходимые для работы Go-сервера
COPY --from=builder /app/keys ./keys/

# Открываем порт, на котором слушает ваш Go-сервер
EXPOSE 2222

# Запускаем ваше Go-приложение (SSH-сервер)
CMD ["./secure_go_server"]