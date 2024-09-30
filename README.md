# Сервис записи и хранения видео с камер видеонаблюдения

# Общие вводные
Для записи видео с камер, сервис использует утилиту gstreamer. Сервис имеет функционал авторизации/аутентификации, собственную систему хранилища файлов, способен записывать в двух режимах: одиночный (идет обыкновенная запись с камеры) и смешанный (на экране появляются две картинки, аудио берется из первого потока).
В качестве дополнительного сервиса для хранения файлов используется Opencast (опционально).

# Getting Started

- Подключиться к сети камер.
- Изменить файл config/config.go под свои параметры (Для работы с Opencast указываем путь к конфигу Opencast, если он не нужен, то оставляем пустым.)
- Создать .env файл, в котором указываются переменные: 
POSTGRES_PASSWORD= (пароль от БД Postgres)
POSTGRES_USER= (имя пользователя БД Postgres)
ADMIN_EMAIL= (логин для главного начального админа)
ADMIN_PASSWORD= (пароль для главного начального админа)

# Usage

Запустить сервис можно с помощью "docker-compose up -d" (не забудьте указать .env переменные).


## Examples

Примеры запросов через Postman. 
Все запросы требуют JWT авторизацию (кроме логина). Существует два типа пользователя: user и admin.
Также к некоторым ручкам доступ имеет только admin.
- [Пользователи](#auth)
- [Камеры](#camera)
- [Запись](#recordings)

### Пользователь <a name="auth"></a>

**Логин:**
```curl
POST http://localhost:8000/login
```

Body:
```json
{
	"email": "email@yandex.ru",
	"password": "password",
}
```

Пример ответа:
200
```json
{
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJlbWFpbCI6ImFkbWluQGV4YW1wbGUuY29tIiwiZXhwIjoxNzI3NzQyOTM0LCJ1aWQiOjEsInVzZXJfdHlwZSI6ImFkbWluIn0.whO0zV7NaZaQRney75hNWpk9QS-yvOhAilV33cxOzI4"
}
```

**Создание пользователя (доступно лишь admin):**
```curl
POST http://localhost:8000/users
```

Body:
```json
{
	"email": "email@yandex.ru",
	"password": "password",
	"user_type": "admin"
}
```

Пример ответа:
200
```json
{
    "id": 2
}
```

**Обновление пароля (доступно лишь admin):**
```curl
PATCH http://localhost:8000/users
```

Body:
```json
{
	"email": "email@yandex.ru",
	"password": "newpassword",
}
```

Пример ответа:
200

**Удаление пользователя (доступно лишь admin):**
```curl
DELETE http://localhost:8000/users
```

Body:
```json
{
	"email": "email@yandex.ru",
}
```

Пример ответа:
200

### Камеры <a name="create-camera"></a>

**Добавление камеры в базу данных (доступно лишь admin):**
```curl
POST http://localhost:8000/cameras
```

Body:
```json
{
	"camera_ip": "192.168.1.2:554",
	"location": "101",
	"has_audio": true
}
```

Пример ответа:
200
```json
{
    "camera_id": "gCTPVmPH5we2xD8vT4NMp",
	"camera_ip": "192.168.1.2:554",
	"location": "101",
	"has_audio": true
}
```

**Получение камер:**
```curl
GET http://localhost:8000/cameras
```

Пример ответа:
200
```json
[
    {
        "camera_id": "gCTPVmPH5we2xD8vT4NMp",
	    "camera_ip": "192.168.1.2:554",
	    "room_number": "101",
	    "has_audio": true
    }
]
```

**Обновление информации о камере (доступно лишь admin):**
```curl
PATCH http://localhost:8000/cameras/gCTPVmPH5we2xD8vT4NMp
```

Body:
```json
{
	"location": "103",
	"has_audio": false
}
```

Пример ответа:
200
```json
{
    "camera_id": "gCTPVmPH5we2xD8vT4NMp",
	"camera_ip": "192.168.1.2:554",
	"room_number": "103",
	"has_audio": false
}
```

**Удаление камеры (доступно лишь admin):**
```curl
DELETE http://localhost:8000/cameras/gCTPVmPH5we2xD8vT4NMp
```

Body:
```json
{
	"camera_id": "103",
	"has_audio": false
}
```

Пример ответа:
200

### Запись (может вестись только с добавленных камер) <a name="recordings"></a>

**Начало обычной одиночной записи:**
```curl
POST http://localhost:8000/recordings/start
```

Body:
```json
{
	"camera_ids": ["gCTPVmPH5we2xD8vT4NMp"]
}
```

Пример ответа:
200
```json
{
    "record_id": "4f2329e4-104a-4d45-a7f8-dc5f1357b17d"
}
```


**Начало смешанной записи:**
```curl
POST http://localhost:8000/recordings/start
```

Body:
```json
{
	"camera_ids": ["gCTPVmPH5we2xD8vT4NMp","hTYPVmPH3we2xD8vT4NMp"]
}
```

Пример ответа:
200
```json
{
    "record_id": "4f2329e4-104a-4d45-a7f8-dc5f1357b17d"
}
```


**Остановка записи:**
```curl
POST http://localhost:8000/recordings/4f2329e4-104a-4d45-a7f8-dc5f1357b17d/stop
```

Пример ответа:
200


**Запланированная запись с указанием времени и продолжнительности (поддерживается как одиночная, так и смешанная запись):**
```curl
POST http://localhost:8000/recordings/schedule
```
Body:
```json
{
    "camera_ids": ["gCTPVmPH5we2xD8vT4NMp","hTYPVmPH3we2xD8vT4NMp"],
	"start_time": "2024-05-22T15:00:00+03:00",
	"duration": "1h"
}
В КОНЦЕ УКАЗЫВАЕТСЯ +3:00 КАК МОСКОВСКИЙ ЧАСОВОЙ ПОЯС

```
Пример ответа:
200


**Получение записей с камеры с лимитом и оффсетом:**
```curl
GET http://localhost:8080/recordings/gCTPVmPH5we2xD8vT4NMp?limit=1&offset=1
```

Пример ответа:
200
```json
[
    {
        "recording_id": "4f2329e4-104a-4d45-a7f8-dc5f1357b17d",
        "camera_ip": "rtsp://admin:Supisor520@173.18.191.62:561/stream1",
        "user_id": 2,
        "start_time": "2024-09-30T18:38:32.23425Z",
        "stop_time": "2024-09-30T18:38:54.568713Z",
        "is_moved": false
    }
]
```

**Перенос записи в видео сервис:**
```curl
POST http://localhost:8080/recordings/4f2329e4-104a-4d45-a7f8-dc5f1357b17d/move
```

Пример ответа:
200

**Удаление записи:**
```curl
DELETE http://localhost:8080/recordings/4f2329e4-104a-4d45-a7f8-dc5f1357b17d
```

Пример ответа:
200