# Используем образ с Go для сборки
FROM golang:1.24 as builder

WORKDIR /app

# Копируем go.mod и go.sum для загрузки зависимостей
COPY go.mod go.sum ./
RUN go mod download

# Копируем все исходники
COPY . .

# Собираем бинарник, компилируем все исходники
RUN go build -o monitor .

# Используем образ debian для запуска
FROM debian:latest

WORKDIR /root/

# Копируем собранный бинарник
COPY --from=builder /app/monitor .

# Даем права на выполнение
RUN chmod +x /root/monitor

ENTRYPOINT ["/root/monitor"]
