# Etapa 1: Compilación
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copiar dependencias y código fuente
COPY go.mod ./
COPY . .

# Compilar binario estático optimizado
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o intercambio .

# Etapa 2: Imagen final ultraligera
FROM alpine:3.21

WORKDIR /app

# Instalar certificados SSL/TLS y zona horaria
RUN apk --no-cache add ca-certificates tzdata

# Crear directorio de datos
RUN mkdir -p /app/data

# Copiar el binario desde la etapa de compilación
COPY --from=builder /app/intercambio /app/intercambio

# Variables de entorno por defecto
ENV PORT=8080
ENV DATA_PATH=/app/data/secretsanta.json
ENV BASE_URL=http://localhost:8080

EXPOSE 8080

VOLUME ["/app/data"]

CMD ["/app/intercambio"]
