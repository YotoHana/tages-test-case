# File Storage gRPC Service

Сервис для загрузки, скачивания и просмотра файлов через gRPC с поддержкой rate limiting.

## Содержание

- [Возможности](#возможности)
- [Технологии](#технологии)
- [Быстрый старт](#быстрый-старт)
- [Использование](#использование)
- [Архитектура](#архитектура)
- [Rate Limiting](#rate-limiting)
- [Тестирование](#тестирование)
- [API](#api)

## Возможности

- **Upload**: Загрузка файлов с использованием client streaming
- **Download**: Скачивание файлов с использованием server streaming
- **List**: Просмотр списка загруженных файлов с метаданными
- **Rate Limiting**: Ограничение количества одновременных подключений
  - Upload/Download: максимум 10 одновременных запросов
  - List: максимум 100 одновременных запросов
- **Streaming**: Эффективная передача больших файлов по частям (64KB chunks)
- **Валидация**: Проверка входных данных и ограничение размера файлов (100MB)
- **Graceful Shutdown**: Корректное завершение работы сервера

## Технологии

- **Go 1.25+**
- **gRPC** - коммуникация клиент-сервер
- **Protocol Buffers** - сериализация данных
- **Semaphore** - rate limiting через буферизованные каналы

## Быстрый старт

### Предварительные требования

```bash
# Go 1.25 или выше
go version

# Protocol Buffers compiler
protoc --version

# Установка protoc-gen-go плагинов
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

### Установка

```bash
# Клонировать репозиторий
git clone https://github.com/YotoHana/tages-test-case.git
cd tages-test-case

# Установить зависимости
make deps

# Сгенерировать код из proto файлов
make proto
```

### Запуск

#### Локально

```bash
# Терминал 1: Запустить сервер
make run-server

# Терминал 2: Использовать клиент
make upload FILE=test.jpg
make list
make download ID=<file_id> OUT=./downloads
```

#### С Docker Compose

```bash
# Запустить сервер в Docker
make docker-compose-up

# Использовать клиент (в другом терминале)
make upload FILE=test.jpg
make list

# Остановить
make docker-compose-down
```

#### С Docker (без compose)

```bash
# Собрать образ
make docker-build

# Запустить контейнер
make docker-run

# Использовать клиент
make upload FILE=test.jpg

# Остановить
make docker-stop
```

## Использование

### Makefile команды

```bash
# Просмотр всех доступных команд
make help

# Генерация proto файлов
make proto
make proto-clean

# Запуск сервера и клиента
make run-server
make run-client ARGS='<command> [args]'

# Быстрые команды для клиента
make upload FILE=path/to/file.jpg
make download ID=abc123 OUT=./output
make list
make test-limits

# Сборка бинарников
make build

# Очистка
make clean

# Установка зависимостей
make deps

# Линтер
make lint
```

### Прямое использование

#### Сервер

```bash
go run ./cmd/server/server.go
```

Сервер запустится на `localhost:50051`

#### Клиент

```bash
# Загрузка файла
go run ./cmd/client/client.go upload path/to/file.jpg

# Скачивание файла
go run ./cmd/client/client.go download <file_id> ./output

# Просмотр списка файлов
go run ./cmd/client/client.go list

# Тестирование rate limits
go run ./cmd/client/client.go test-limits
```

### Примеры использования

#### Загрузка файла

```bash
$ make upload FILE=photo.jpg
Uploading photo.jpg (2458624 bytes)...
Progress: 100.0%
Upload successful!
File ID: a3f5c892d1e4b6c7
```

#### Просмотр списка файлов

```bash
$ make list
List Files:
ID: a3f5c892d1e4b6c7 | FileName: photo.jpg | Created_At: 2024-01-20 10:30:45 | Updated_At: 2024-01-20 10:30:45
ID: b2e4d3a1c5f6e7d8 | FileName: document.pdf | Created_At: 2024-01-20 10:32:10 | Updated_At: 2024-01-20 10:32:10
```

#### Скачивание файла

```bash
$ make download ID=a3f5c892d1e4b6c7 OUT=./downloads
Downloading file a3f5c892d1e4b6c7...
Received: 2458624 bytes
Download successful!
```

## Архитектура

```
tages-test-case/
├── api/
│   └── proto/
│       ├── file_service.proto      # Proto описание API
│       ├── file_service.pb.go      # Сгенерированные типы
│       └── file_service_grpc.pb.go # Сгенерированный gRPC код
├── cmd/
│   ├── server/
│   │   └── server.go               # Точка входа сервера
│   └── client/
│       └── client.go               # CLI клиент
├── internal/
│   ├── api/
│   │   └── handler.go              # gRPC handlers
│   ├── storage/
│   │   └── storage.go              # Работа с файловой системой
│   └── semaphore/
│       └── semaphore.go            # Rate limiting
├── uploads/                         # Директория для загруженных файлов
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

### Компоненты

#### Server

- **gRPC Server**: Обрабатывает входящие запросы на порту 50051
- **Interceptors**: Middleware для rate limiting и логирования
- **Handlers**: Реализация методов Upload, Download, List
- **Storage**: Управление файлами на диске

#### Client

- **CLI Interface**: Удобный интерфейс командной строки
- **Streaming Support**: Отправка и получение файлов по частям
- **Error Handling**: Обработка различных gRPC статус-кодов

#### Rate Limiting

- **Semaphore**: Реализация через буферизованные каналы
- **Per-method limits**: Разные лимиты для разных операций
- **Non-blocking**: Использование TryAcquire для немедленного отклонения

## Rate Limiting

### Настройки

| Операция | Лимит | Описание |
|----------|-------|----------|
| **Upload** | 10 | Максимум 10 одновременных загрузок |
| **Download** | 10 | Максимум 10 одновременных скачиваний |
| **List** | 100 | Максимум 100 одновременных запросов списка |

**Примечание**: Upload и Download используют **общий** лимитер (10 запросов суммарно).

### Как это работает

Rate limiting реализован через **семафор** на основе буферизованных каналов:

```go
type Semaphore struct {
    slots chan struct{} // Буферизованный канал
}

// Создание семафора с 10 слотами
sem := NewSemaphore(10)

// Попытка взять слот (неблокирующая)
if !sem.TryAcquire() {
    return codes.ResourceExhausted // Лимит превышен
}
defer sem.Release() // Освобождаем слот после завершения
```

**Принцип работы:**

1. Каждый запрос пытается "взять" слот из семафора
2. Если слоты есть (< 10) - запрос выполняется
3. Если слотов нет (= 10) - запрос отклоняется с ошибкой `ResourceExhausted`
4. После завершения запроса слот освобождается через `defer`

### Поведение при превышении лимита

```bash
$ make upload FILE=test.jpg
Rate limit exceeded: Too many concurrent upload requests.
Please try again in a few seconds.
```

## Тестирование

### Автоматическое тестирование rate limits

```bash
make test-limits
```

**Что тестируется:**

1. **Upload limit**: Отправляет 15 одновременных запросов, ожидает ~10 успешных
2. **List limit**: Отправляет 105 одновременных запросов, ожидает ~100 успешных

**Пример вывода:**

```
Testing rate limits...
Test 1: Upload rate limit (max 10 concurrent)
Sending 15 concurrent upload requests...

Request  1: SUCCESS
Request  2: SUCCESS
Request  3: SUCCESS
Request  4: SUCCESS
Request  5: SUCCESS
Request  6: SUCCESS
Request  7: SUCCESS
Request  8: SUCCESS
Request  9: SUCCESS
Request 10: SUCCESS
Request 11: RATE LIMITED
Request 12: RATE LIMITED
Request 13: RATE LIMITED
Request 14: RATE LIMITED
Request 15: RATE LIMITED

Results (completed in 1.2s):
Successful: 10 (expected ~10)
Rate limited: 5 (expected ~5)
Other errors: 0 (expected 0)
Upload rate limiting works correctly!

Test 2: List rate limit (max 100 concurrent)
Sending 105 concurrent list requests...

Results (completed in 0.3s):
Successful: 100 (expected ~100)
Rate limited: 5 (expected ~5)
List rate limiting works correctly!

Rate limit testing complete!
```

### Ручное тестирование

#### Тест 1: Базовый функционал

```bash
# 1. Загрузить файл
make upload FILE=test.jpg

# 2. Проверить что файл появился в списке
make list

# 3. Скачать файл по ID
make download ID=<file_id> OUT=./downloads

# 4. Проверить что файл скачался
ls -lh ./downloads
```

#### Тест 2: Rate limiting

```bash
# Запустить несколько загрузок одновременно
for i in {1..15}; do
    make upload FILE=test.jpg &
done
wait

# Ожидаемое: 10 успешных, 5 с ошибкой rate limit
```

#### Тест 3: Обработка ошибок

```bash
# Попытка скачать несуществующий файл
make download ID=invalid_id OUT=./output
# Ожидаемое: "File not found"

# Попытка загрузить несуществующий файл
make upload FILE=nonexistent.jpg
# Ожидаемое: "Failed to open file"
```

## API

### gRPC Service Definition

```protobuf
service FileService {
    rpc Upload(stream UploadRequest) returns (UploadResponse);
    rpc Download(DownloadRequest) returns (stream DownloadResponse);
    rpc List(ListRequest) returns (ListResponse);
}
```

### Upload

**Client Streaming RPC**: Клиент отправляет файл по частям.

**Request:**
```protobuf
message UploadRequest {
    oneof data {
        string filename = 1;  // Первое сообщение - имя файла
        bytes chunk = 2;      // Остальные - данные
    }
}
```

**Response:**
```protobuf
message UploadResponse {
    string id = 1;  // Уникальный ID загруженного файла
}
```

**Процесс:**
1. Клиент отправляет filename в первом сообщении
2. Клиент отправляет данные файла чанками по 64KB
3. Сервер сохраняет файл и возвращает уникальный ID

**Ограничения:**
- Filename не может быть пустым
- Файл не может быть пустым (0 байт)

### Download

**Server Streaming RPC**: Сервер отправляет файл по частям.

**Request:**
```protobuf
message DownloadRequest {
    string id = 1;  // ID файла для скачивания
}
```

**Response:**
```protobuf
message DownloadResponse {
    oneof payload {
        FileInfo info = 1;   // Первое сообщение - метаданные
        bytes chunk = 2;     // Остальные - данные
    }
}
```

**Процесс:**
1. Клиент запрашивает файл по ID
2. Сервер отправляет метаданные (имя файла)
3. Сервер отправляет данные файла чанками по 64KB

### List

**Unary RPC**: Простой запрос-ответ.

**Request:**
```protobuf
message ListRequest {}  // Пустой запрос
```

**Response:**
```protobuf
message ListResponse {
    message Item {
        string id = 1;
        string name = 2;
        google.protobuf.Timestamp created_at = 3;
        google.protobuf.Timestamp updated_at = 4;
    }
    repeated Item items = 1;
}
```

**Процесс:**
1. Клиент запрашивает список файлов
2. Сервер возвращает все файлы с метаданными

## Обработка ошибок

Сервис использует стандартные gRPC статус-коды:

| Код | Значение | Когда возникает |
|-----|----------|-----------------|
| `InvalidArgument` | Некорректные входные данные | Пустой filename, пустой ID, файл > 100MB |
| `NotFound` | Ресурс не найден | Файл с указанным ID не существует |
| `ResourceExhausted` | Лимит превышен | Слишком много одновременных запросов |
| `Internal` | Внутренняя ошибка | Ошибка записи на диск, IO error |
| `DeadlineExceeded` | Превышено время ожидания | Операция заняла слишком много времени |
| `Unavailable` | Сервис недоступен | Сервер не запущен или недоступен |

### Примеры ошибок

```bash
# Rate limit
Rate limit exceeded: Too many concurrent upload requests.
Please try again in a few seconds.

# Файл не найден
File not found: file with id 'invalid_id' not found

# Невалидный аргумент
Invalid request: filename cannot be empty

# Файл слишком большой
Invalid request: file size exceeds maximum allowed size of 104857600 bytes
```

## Хранение файлов

### Формат имени файла

Файлы сохраняются в формате: `{id}_{original_name}`

```
uploads/
├── a3f5c892d1e4b6c7_photo.jpg
└── c1f2e3d4a5b6c7d8_image.png
```

**Где:**
- `id` - uuid
- `original_name` - оригинальное имя файла

### Генерация ID

```go
    id = uuid.NewString()
	resultName := strings.Join([]string{id, fileName}, "_")
	file, err = os.Create(filepath.Join(storageRoot, resultName))
```

**Преимущества:**
- Уникальность гарантирована
- Невозможно угадать другие ID
- Легко искать файл по ID (имя начинается с ID)

## Безопасность

### Реализованные меры

- Валидация имени файла (проверка на path traversal)
- Уникальные имена файлов (защита от перезаписи)
- Rate limiting (защита от DDoS)
- Graceful shutdown (корректное завершение операций)
- Cleanup при ошибках (удаление частично загруженных файлов)

### Что можно улучшить

- TLS/SSL для шифрования передачи данных
- Аутентификация и авторизация пользователей
- Проверка MIME типов загружаемых файлов
- Антивирусное сканирование
- Квоты на пользователя
- Логирование всех операций
- Метрики и мониторинг

## Производительность

### Характеристики

- **Chunk size**: 64KB (баланс между скоростью и памятью)
- **Concurrent uploads**: 10
- **Concurrent downloads**: 10
- **Concurrent list**: 100

### Оптимизация

Для увеличения производительности:

1. **Увеличить chunk size** (в `internal/api/handler.go`):
```go
const chunkSize = 256 * 1024 // 256KB для быстрой сети
```

2. **Увеличить лимиты** (в `internal/api/handler.go`):
```go
uploadSemaphore: NewSemaphore(50)  // 50 вместо 10
```

3. **Использовать SSD** для хранения файлов

## Поддержка

Если у вас возникли вопросы или проблемы:

1. Проверьте что сервер запущен: `make run-server`
2. Проверьте что порт 50051 не занят: `lsof -i :50051`
3. Проверьте что proto файлы сгенерированы: `make proto`
4. Посмотрите логи сервера для деталей ошибок

## Дополнительные ресурсы

- [gRPC Documentation](https://grpc.io/docs/)
- [Protocol Buffers Guide](https://protobuf.dev/)
- [Go gRPC Tutorial](https://grpc.io/docs/languages/go/quickstart/)