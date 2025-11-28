# Pod API

HTTP‑сервис на Echo, который:
- отправляет текстовые запросы в GigaChat и возвращает ответ в унифицированном виде;
- принимает изображение с промптом, временно хранит картинку в памяти, передаёт ссылку в OpenAI Vision и отдаёт описания;
- выдаёт сохранённые изображения по UUID (с удалением после скачивания);
- предоставляет healthcheck и экспозицию метрик.

## Точка входа
- `cmd/main.go` настраивает логирование (zerolog), читает конфигурацию из окружения, создаёт реестр метрик и сервер Echo.
- Регистрируются базовые ручки `/ping`, `/metrics`, `/metrics.json`.
- Инициализируются клиенты GigaChat и OpenAI, in-memory репозиторий изображений и HTTP‑обработчики из `pkg/api`.
- Сервер слушает `HOST:PORT`; `BASE_URL` используется для формирования абсолютных ссылок на картинки.

## Конфигурация
Переменные окружения парсятся в `pkg/config` (поддерживается `.env`):

| Переменная | Назначение | Значение по умолчанию |
| --- | --- | --- |
| `PORT` | Порт HTTP‑сервера | `8080` |
| `HOST` | Адрес для bind | `0.0.0.0` |
| `BASE_URL` | Базовый URL для ссылок на изображения (если пусто — относительные пути) | `""` |
| `OPENAI_URL` | Базовый URL OpenAI | `https://api.aitunnel.ru/v1` |
| `OPENAI_BASIC_KEY` | Ключ для OpenAI (Basic) | — (обязательно) |
| `OPENAI_MODEL` | Модель OpenAI | — (обязательно) |
| `OPENAI_REQUEST_TIMEOUT` | Таймаут запросов к OpenAI | `30s` |
| `GIGACHAT_URL` | Базовый URL API GigaChat | `https://gigachat.devices.sberbank.ru/api/v1` |
| `GIGACHAT_AUTH_URL` | Базовый URL OAuth для GigaChat | `https://ngw.devices.sberbank.ru:9443/api/v2` |
| `GIGACHAT_MODEL` | Модель GigaChat (`GigaChat-2`, `GigaChat-2-Pro`, `GigaChat-2-Max`) | `GigaChat-2` |
| `GIGACHAT_SCOPE` | OAuth scope | `GIGACHAT_API_PERS` |
| `GIGACHAT_TOKEN_REFRESH_LEEWAY_SECONDS` | Лиюэй обновления токена | `10` |
| `GIGACHAT_BASIC_KEY` | Base64(client_id:client_secret) для OAuth | — (обязательно) |
| `GIGACHAT_ROOT_CA_URL` | URL PEM‑корневого сертификата для TLS | `https://gu-st.ru/content/lending/russian_trusted_root_ca_pem.crt` |
| `GIGACHAT_MAX_TOKENS` | Лимит `max_tokens` в чат‑ответах | `1024` |
| `IMAGE_TTL` | Время жизни изображений в памяти | `30s` |

## Ручки
- `GET /ping` — healthcheck, возвращает `pong`.
- `GET /metrics` / `GET /metrics.json` — счётчики в текстовом или JSON виде.
- `POST /api/v1/chat/text`
  - Тело: JSON `{ "text": "<ваш вопрос>" }`.
  - Логика: запрос уходит в GigaChat (TextModel); ответ нормализуется в общий формат.
  - Ответ: `{"items":[{"description":"<ответ модели>"}]}`. Пустое тело — 400, ошибки модели — 500.
- `POST /api/v1/chat/image`
  - Тело: `multipart/form-data` с полями `image` (PNG/JPEG) и `text` (промпт).
  - Логика: проверяет тип файла, сохраняет байты в памяти с TTL (`IMAGE_TTL`), генерирует ссылку `/api/v1/images/{id}` (с `BASE_URL`, если задан), передаёт промпт и ссылку в OpenAI Vision и собирает ответ.
  - Ответ: `{"items":[{"name":"<модель>","description":"<ответ>","mainImageUrl":"<url>","carouselImageUrls":["<url>"]}]}`. Ошибки чтения/валидации — 400, ошибки модели — 500.
- `GET /api/v1/images/{id}?callback=<url>`
  - Логика: отдаёт сохранённое изображение по UUID с типом `image/png` или `image/jpeg`; после успешной выдачи удаляет объект из памяти.
  - Дополнительно: если передан `callback`, после удаления отправляется POST на указанный URL с телом `{"id":"<uuid>","status":"delivered"}`. Не найдено — 404.

Актуальная схема OpenAPI лежит в `swagger/openapi.yml`; генерация Go‑клиентов/серверов — через `make gen` (oapi-codegen + easyjson).

## Запуск и сборка
- Локально: `go run ./cmd` (или `go build -o bin/pod_api ./cmd`).
- Генерация кода по Swagger: `make gen`.
- Сборка: `make build`. Требования: Go 1.25+, доступ к интернету для загрузки Root CA GigaChat.

## Наблюдаемость и вспомогательное
- Логи — zerolog в консольном формате (`pkg/logging`).
- Мидлвар `pkg/middleware/request_logging` проставляет `X-Request-ID`, логирует запросы и инкрементирует метрики `http_requests_total` / `http_requests_errors_total`.
- Метрики в памяти + зеркалирование в OpenTelemetry (`pkg/metrics`); изображения — в памяти с TTL (`pkg/repository/image`).

## Примеры запросов
- Текст: `curl -X POST http://localhost:8080/api/v1/chat/text -H "Content-Type: application/json" -d '{"text":"describe this"}'`
- Картинка: `curl -X POST http://localhost:8080/api/v1/chat/image -F "text=what is on photo" -F "image=@sample.jpg"`
- Картинка по id: `curl -L http://localhost:8080/api/v1/images/<uuid>`
- Метрики JSON: `curl http://localhost:8080/metrics.json`

## Пример .env
```
PORT=8080
HOST=0.0.0.0
BASE_URL=http://localhost:8080
OPENAI_URL=https://api.aitunnel.ru/v1
OPENAI_BASIC_KEY=xxx
OPENAI_MODEL=gpt-4o-mini
OPENAI_REQUEST_TIMEOUT=30s
GIGACHAT_URL=https://gigachat.devices.sberbank.ru/api/v1
GIGACHAT_AUTH_URL=https://ngw.devices.sberbank.ru:9443/api/v2
GIGACHAT_MODEL=GigaChat-2
GIGACHAT_SCOPE=GIGACHAT_API_PERS
GIGACHAT_TOKEN_REFRESH_LEEWAY_SECONDS=10
GIGACHAT_BASIC_KEY=yyy
GIGACHAT_ROOT_CA_URL=https://gu-st.ru/content/lending/russian_trusted_root_ca_pem.crt
GIGACHAT_MAX_TOKENS=1024
IMAGE_TTL=30s
```

## Ограничения и ошибки
- Поддерживаемые изображения: `image/png`, `image/jpeg`. Пустое тело или неправильный тип — 400.
- Не найдено изображение: 404 (`/api/v1/images/{id}`).
- Ошибки моделей или внутренние сбои — 500.
- TTL для картинок задаётся `IMAGE_TTL`; после выдачи `/api/v1/images/{id}` удаляет объект сразу.

## Архитектура коротко
- `cmd/main.go` — wiring: логирование → конфиг → метрики → Echo → middleware → регистрация OpenAPI‑хендлеров.
- Клиенты: `pkg/clients/gigachat` (чат‑ответы), `pkg/clients/openai` (vision).
- Бизнес‑логика API: `pkg/api/handlers.go`.
- Хранилище изображений: `pkg/repository/image` (in-memory с TTL).
- Метрики и логирование: `pkg/metrics`, `pkg/middleware/request_logging`, `pkg/logging`.

## Генерация кода
- Swagger: `swagger/openapi.yml`.
- Генераторы: `make gen` (ставит `oapi-codegen`, `easyjson`, затем `go generate ./...`).
- Сгенерированные файлы: `pkg/apigen/openapi/*`, `pkg/apigen/gigachat/*`.

## Безопасность и прод‑запуск
- Изображения хранятся только в памяти; персистентного диска нет.
- `BASE_URL` обязателен в проде, если клиенты читают картинки по внешнему адресу.
- Нужен доступ к интернету для загрузки Root CA GigaChat при старте.
- Проверьте открытые порты и переменные окружения перед деплоем.
